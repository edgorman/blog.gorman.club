package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

var _ repository.Embedder = (*Embedder)(nil)

// EmbedderConfig names the text-embedding model to call and the size of vector to ask it for.
type EmbedderConfig struct {
	Config
	// Dimension is the length of every vector returned. It must match the vector index on the
	// embeddings collection (infrastructure/env/firestore.tf), which is why both come from the same
	// Terraform variable.
	Dimension int
}

// Embedder implements repository.Embedder with the platform's :predict method, which both the
// text-embedding-* and gemini-embedding-* models serve. Authentication is the same ADC client the
// Assistant uses.
type Embedder struct {
	cfg EmbedderConfig

	once   sync.Once
	client *http.Client
	err    error
}

// NewEmbedder returns an Embedder for cfg. It performs no I/O.
func NewEmbedder(cfg EmbedderConfig) *Embedder {
	if cfg.Location == "" {
		cfg.Location = "global"
	}
	return &Embedder{cfg: cfg}
}

// Configured reports whether there is a model to call at all.
func (e *Embedder) Configured() bool {
	return e.cfg.ProjectID != "" && e.cfg.Model != "" && e.cfg.Dimension > 0
}

func (e *Embedder) Model() string { return e.cfg.Model }

type predictRequest struct {
	Instances  []embedInstance `json:"instances"`
	Parameters embedParameters `json:"parameters"`
}

type embedInstance struct {
	Content string `json:"content"`
	// TaskType is RETRIEVAL_DOCUMENT for a post, which is what gets found, whether by another post
	// (related posts) or by a search query, which EmbedQuery sends as RETRIEVAL_QUERY.
	TaskType string `json:"task_type"`
}

type embedParameters struct {
	OutputDimensionality int `json:"outputDimensionality"`
	// AutoTruncate cuts a post longer than the model's input limit rather than refusing it: its
	// opening is a fair stand-in for what it is about.
	AutoTruncate bool `json:"autoTruncate"`
}

type predictResponse struct {
	Predictions []struct {
		Embeddings struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	} `json:"predictions"`
}

// Embed returns the vector of text as a document: a post, to be found.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text, "RETRIEVAL_DOCUMENT")
}

// EmbedQuery returns the vector of text as a search query, comparable with Embed's document vectors
// of the same model.
func (e *Embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text, "RETRIEVAL_QUERY")
}

func (e *Embedder) embed(ctx context.Context, text, taskType string) ([]float32, error) {
	if !e.Configured() {
		return nil, fmt.Errorf("embedder is not configured")
	}
	e.once.Do(func() { e.client, e.err = adcClient(ctx, e.cfg.HTTPClient) })
	if e.err != nil {
		return nil, e.err
	}

	body, err := json.Marshal(predictRequest{
		Instances:  []embedInstance{{Content: text, TaskType: taskType}},
		Parameters: embedParameters{OutputDimensionality: e.cfg.Dimension, AutoTruncate: true},
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.modelURL("predict"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := e.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call embedding model: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		// As in generate, the provider's message is not repeated: it can quote the post back.
		return nil, fmt.Errorf("embedding model answered %d%s", response.StatusCode, failureDetail(payload))
	}

	var decoded predictResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(decoded.Predictions) != 1 || len(decoded.Predictions[0].Embeddings.Values) != e.cfg.Dimension {
		return nil, fmt.Errorf("embedding model returned no %d-dimension vector", e.cfg.Dimension)
	}
	return decoded.Predictions[0].Embeddings.Values, nil
}
