package embedclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbed_ParsesSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("server: failed to decode request: %v", err)
		}
		if req.Text != "white cast free sunscreen" {
			t.Errorf("server received text %q, want the original query text", req.Text)
		}
		json.NewEncoder(w).Encode(embedResponse{Vector: []float32{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	c := New(server.URL)
	vec, err := c.Embed(context.Background(), "white cast free sunscreen")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(vec) != 3 || vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
		t.Errorf("Embed = %v, want [0.1 0.2 0.3]", vec)
	}
}

func TestEmbed_ServiceErrorIsSurfaced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(embedResponse{Error: "'text' must be a non-empty string"})
	}))
	defer server.Close()

	c := New(server.URL)
	_, err := c.Embed(context.Background(), "")
	if err == nil {
		t.Fatal("Embed returned nil error for a 400 response, want an error")
	}
}

func TestEmbed_ContextCancellationStopsTheRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{Vector: []float32{1}})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New(server.URL)
	_, err := c.Embed(ctx, "text")
	if err == nil {
		t.Fatal("Embed returned nil error with an already-canceled context, want an error")
	}
}
