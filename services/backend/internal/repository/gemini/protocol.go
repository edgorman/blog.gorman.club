package gemini

import (
	"bytes"
	"encoding/json"
	"strings"
)

// The generateContent wire format, hand-written rather than pulled in with a provider SDK. The
// request this service makes is a small and stable corner of that API - contents, one system
// instruction, one set of function declarations - and spelling it out here costs a few dozen lines
// against a dependency tree larger than the rest of this backend put together. It also keeps the
// adapter testable against an httptest server rather than against a client that has to be
// persuaded to talk to one.

// Gemini names the two conversation roles "user" and "model". Tool results are sent as the user's
// turn, which is the protocol's convention rather than a claim that a person produced them.
const (
	roleUser  = "user"
	roleModel = "model"
)

type generateRequest struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	Tools             []tool            `json:"tools,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type generationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

// part is a union: exactly one of the fields below is set on a part this package builds.
// Everything is omitempty because sending a part with two of them present is a request the API
// rejects.
//
// A part that arrived from the model also keeps the exact bytes it arrived as, and is sent back as
// those bytes rather than as a re-encoding of the three fields modelled here. That matters because
// a turn has to be replayed into the conversation before its tool results can answer it, and the
// model puts things on its own parts that this struct does not know about - a thinking model
// returns a thoughtSignature, "an opaque signature for the thought so it can be reused in
// subsequent requests", and re-encoding the turn would drop it. Losing a field the model requires
// back is not a difference the type system can catch, so the fix is not to model more fields but
// to stop paraphrasing what the model said.
type part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`

	// raw is set only on a part decoded from a response, and is unexported so it is never a field
	// of the JSON itself.
	raw json.RawMessage
}

// partFields mirrors part without its methods, so the marshalling below can fall back to the
// default encoding of the same fields without recursing into itself.
type partFields part

func (p part) MarshalJSON() ([]byte, error) {
	if len(p.raw) > 0 {
		return p.raw, nil
	}
	return json.Marshal(partFields(p))
}

func (p *part) UnmarshalJSON(data []byte) error {
	var fields partFields
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	*p = part(fields)
	p.raw = bytes.Clone(data)
	return nil
}

// functionCall is the model asking for a tool to be run. Args stays raw so each tool decodes its
// own arguments into a shape it declared, rather than everything going through map[string]any.
type functionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type functionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type tool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
}

type functionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  schema `json:"parameters"`
}

// schema is the subset of OpenAPI that function parameters need here: an object of strings. Type
// names are the API's uppercase enum form.
type schema struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
}

type generateResponse struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

// blockReason names why a request produced no candidate, when the API said.
func (r generateResponse) blockReason() string {
	return r.PromptFeedback.BlockReason
}

// content is the model's turn, to be appended to the conversation before its tool results are.
// Its parts carry the bytes they arrived as (see part), so replaying the turn hands back exactly
// what the model said rather than this package's reading of it.
func (r generateResponse) content() content {
	turn := r.Candidates[0].Content
	// The API sets the role itself; it is filled in here only for a response that left it out, so
	// the replayed turn is never attributed to the wrong speaker.
	if turn.Role == "" {
		turn.Role = roleModel
	}
	return turn
}

// parts splits the first candidate into what the model said and what it asked to run. A single
// turn can hold both - "shortening the intro" alongside the call that shortens it - so neither is
// treated as excluding the other.
func (r generateResponse) parts() (string, []functionCall) {
	var (
		text  []string
		calls []functionCall
	)
	for _, p := range r.Candidates[0].Content.Parts {
		if trimmed := strings.TrimSpace(p.Text); trimmed != "" {
			text = append(text, trimmed)
		}
		if p.FunctionCall != nil {
			calls = append(calls, *p.FunctionCall)
		}
	}
	return strings.Join(text, "\n\n"), calls
}
