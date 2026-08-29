// Package vertex talks to Gemini on Vertex AI, which is the same GCP project the rest of this
// deployment already lives in. That is the whole reason it is Vertex rather than the Generative
// Language API: Vertex authenticates with the Cloud Run runtime service account over Application
// Default Credentials, so there is no API key to mint, store in Secret Manager, rotate, or leak -
// exactly the argument that put GitHub Actions on Workload Identity Federation.
package vertex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// cloudPlatformScope is what a Vertex call is authorized by; the runtime service account holds
	// roles/aiplatform.user on the project (see infrastructure/env/cloud_run.tf).
	cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"
	// requestTimeout bounds one call to the model. A request that has not answered by then has
	// already outlasted anyone waiting on the other end of the editor.
	requestTimeout = 60 * time.Second
	// replyTimeout bounds a whole turn, however many rounds of tool calls it takes. Without it the
	// worst case is maxToolRounds requests of requestTimeout each, which outlasts Cloud Run's own
	// request timeout - the caller would be cut off with nothing stored rather than told.
	replyTimeout = 2 * time.Minute
	// maxToolRounds bounds the call-and-respond loop below. A model that has not finished editing
	// after this many rounds is looping rather than working, and every round is another billed
	// request with the whole conversation in it.
	maxToolRounds = 6
	// maxReplyLength is the longest reply that is kept, in runes. The model is told to be brief;
	// this is what makes it so, since a reply longer than a chat message can hold could not be
	// stored at all (see entity.MaxChatMessageLength).
	maxReplyLength = entity.MaxChatMessageLength
)

var _ repository.Assistant = (*Assistant)(nil)

// Config names the model to call and where it lives.
type Config struct {
	// ProjectID is the GCP project Vertex is billed to and called through. Empty disables the
	// assistant entirely, in the same way an empty client ID disables authentication.
	ProjectID string
	// Location is the Vertex region, e.g. "europe-west1", or "global" for the multi-region
	// endpoint. Model availability is regional, so this is the knob that moves a deployment onto
	// an endpoint that actually serves the model below.
	Location string
	// Model is the Vertex model id, e.g. "gemini-3.7-flash". It is configuration rather than a
	// constant because model ids come and go far faster than this service is redeployed.
	Model string
	// BaseURL overrides the Vertex host. It exists for tests, which point it at an httptest
	// server; a deployment leaves it empty and gets the host Location implies.
	BaseURL string
	// HTTPClient overrides the authenticated client built from Application Default Credentials,
	// for the same reason.
	HTTPClient *http.Client
}

// Assistant implements repository.Assistant against Gemini on Vertex AI.
type Assistant struct {
	cfg Config

	// The credentials are resolved on first use rather than at construction, so a backend started
	// somewhere without ADC - a developer's laptop, a test - still serves every other route and
	// fails only the assistant, with the reason.
	once   sync.Once
	client *http.Client
	err    error
}

// NewAssistant returns an Assistant for cfg. It performs no I/O and never fails: a configuration
// that cannot reach a model is reported by Reply as repository.ErrAssistantNotConfigured, which is
// how repository.TokenVerifier reports the same situation.
func NewAssistant(cfg Config) *Assistant {
	if cfg.Location == "" {
		cfg.Location = "global"
	}
	return &Assistant{cfg: cfg}
}

// Configured reports whether this deployment has a model to call at all.
func (a *Assistant) Configured() bool {
	return a.cfg.ProjectID != "" && a.cfg.Model != ""
}

// httpClient resolves Application Default Credentials once and reuses the client after, so a token
// is fetched from the metadata server and refreshed by the transport rather than per request.
func (a *Assistant) httpClient(ctx context.Context) (*http.Client, error) {
	a.once.Do(func() {
		if a.cfg.HTTPClient != nil {
			a.client = a.cfg.HTTPClient
			return
		}

		source, err := google.DefaultTokenSource(ctx, cloudPlatformScope)
		if err != nil {
			a.err = fmt.Errorf("resolve application default credentials: %w", err)
			return
		}
		client := oauth2.NewClient(context.WithoutCancel(ctx), source)
		client.Timeout = requestTimeout
		a.client = client
	})
	return a.client, a.err
}

// endpoint is the generateContent URL for the configured model. The global endpoint is not
// prefixed with its location, unlike every regional one.
func (a *Assistant) endpoint() string {
	base := a.cfg.BaseURL
	if base == "" {
		host := "https://aiplatform.googleapis.com"
		if a.cfg.Location != "global" {
			host = fmt.Sprintf("https://%s-aiplatform.googleapis.com", a.cfg.Location)
		}
		base = host
	}

	return fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		strings.TrimSuffix(base, "/"), a.cfg.ProjectID, a.cfg.Location, a.cfg.Model)
}

// Reply runs one turn of the conversation, including however many rounds of tool calls the model
// needs to make its edits.
//
// The draft is edited in memory and handed back; nothing here writes to Firestore, and the tools
// the model is given can only change a post's title and body (see entity.Draft). So the worst a
// misbehaving model can do is write a bad post - it cannot publish a private one, reassign it, or
// touch a post other than the one being discussed.
func (a *Assistant) Reply(ctx context.Context, req repository.AssistantRequest) (repository.AssistantReply, error) {
	if !a.Configured() {
		return repository.AssistantReply{}, repository.ErrAssistantNotConfigured
	}

	client, err := a.httpClient(ctx)
	if err != nil {
		return repository.AssistantReply{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, replyTimeout)
	defer cancel()

	draft := req.Draft
	contents := conversation(req)

	var (
		edits []entity.ChatEdit
		said  []string
	)

	for range maxToolRounds {
		// The instructions carry the draft as it stands, so a second round sees the edits the
		// first one made rather than the post as it was when the turn started.
		response, err := a.generate(ctx, client, instructions(draft), contents)
		if err != nil {
			return repository.AssistantReply{}, err
		}

		text, calls := response.parts()
		if text != "" {
			said = append(said, text)
		}
		if len(calls) == 0 {
			break
		}

		// The model's own turn has to go back into the conversation before its results do, or the
		// results answer a question the transcript never shows it asking.
		contents = append(contents, response.content())

		results := make([]part, 0, len(calls))
		for _, call := range calls {
			edit, result := apply(&draft, call)
			if edit.Tool != "" {
				edits = append(edits, edit)
			}
			results = append(results, part{FunctionResponse: &functionResponse{
				Name:     call.Name,
				Response: map[string]any{"result": result},
			}})
		}
		contents = append(contents, content{Role: roleUser, Parts: results})
	}

	return repository.AssistantReply{
		Text:  truncate(strings.Join(said, "\n\n"), maxReplyLength),
		Draft: draft,
		Edits: edits,
	}, nil
}

// generate performs one generateContent call.
func (a *Assistant) generate(ctx context.Context, client *http.Client, system string, contents []content) (generateResponse, error) {
	body, err := json.Marshal(generateRequest{
		Contents:          contents,
		SystemInstruction: &content{Parts: []part{{Text: system}}},
		Tools:             []tool{{FunctionDeclarations: declarations}},
		GenerationConfig:  &generationConfig{Temperature: 0.4, MaxOutputTokens: 8192},
	})
	if err != nil {
		return generateResponse{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint(), bytes.NewReader(body))
	if err != nil {
		return generateResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return generateResponse{}, fmt.Errorf("call vertex: %w", err)
	}
	defer response.Body.Close()

	// Bounded so a wrong endpoint answering with something enormous cannot be read into memory
	// whole; a real response is orders of magnitude smaller.
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return generateResponse{}, err
	}
	if response.StatusCode != http.StatusOK {
		// The status is what a caller acts on, so the provider's message is summarized rather than
		// forwarded - it can carry the request back verbatim, and the request holds the post.
		return generateResponse{}, fmt.Errorf("vertex returned %d", response.StatusCode)
	}

	var decoded generateResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return generateResponse{}, fmt.Errorf("decode vertex response: %w", err)
	}
	if len(decoded.Candidates) == 0 {
		// No candidate at all means the request or the answer was refused outright, which is the
		// one failure worth naming: it is the model declining rather than anything being broken.
		if reason := decoded.blockReason(); reason != "" {
			return generateResponse{}, fmt.Errorf("vertex returned no candidates (%s)", reason)
		}
		return generateResponse{}, fmt.Errorf("vertex returned no candidates")
	}
	return decoded, nil
}

// truncate cuts text to at most limit runes, so a reply that outruns what a chat message can hold
// is shortened rather than rejected.
func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}
