package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedSendsPredictAndReadsVector(t *testing.T) {
	var got predictRequest
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"predictions":[{"embeddings":{"values":[0.1,0.2,0.3]}}]}`))
	}))
	defer server.Close()

	e := NewEmbedder(EmbedderConfig{
		Config:    Config{Model: "m", ProjectID: "p", Location: "europe-west1", BaseURL: server.URL, HTTPClient: server.Client()},
		Dimension: 3,
	})
	vector, err := e.Embed(context.Background(), "title\n\nbody")
	if err != nil {
		t.Fatal(err)
	}
	if len(vector) != 3 || vector[2] != 0.3 {
		t.Errorf("vector = %v", vector)
	}
	if path != "/v1/projects/p/locations/europe-west1/publishers/google/models/m:predict" {
		t.Errorf("path = %s", path)
	}
	if len(got.Instances) != 1 || got.Instances[0].Content != "title\n\nbody" || got.Parameters.OutputDimensionality != 3 {
		t.Errorf("request = %+v", got)
	}
}

// A vector of the wrong size could never be written to the index, so it is refused here instead.
func TestEmbedRejectsWrongDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"predictions":[{"embeddings":{"values":[0.1]}}]}`))
	}))
	defer server.Close()

	e := NewEmbedder(EmbedderConfig{
		Config:    Config{Model: "m", ProjectID: "p", BaseURL: server.URL, HTTPClient: server.Client()},
		Dimension: 3,
	})
	if _, err := e.Embed(context.Background(), "x"); err == nil {
		t.Error("want an error for a 1-dimension vector")
	}
}
