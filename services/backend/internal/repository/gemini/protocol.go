package gemini

import (
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

// part is a union: exactly one field is set. Everything is omitempty because sending a part with
// two of the three present is a request the API rejects.
type part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
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
func (r generateResponse) content() content {
	turn := r.Candidates[0].Content
	turn.Role = roleModel
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
