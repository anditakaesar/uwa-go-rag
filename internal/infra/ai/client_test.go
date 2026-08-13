package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbed(t *testing.T) {
	t.Run("return float32 vector from configured model and dimensions", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			err := json.NewDecoder(r.Body).Decode(&gotBody)
			require.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{
				"object": "list",
				"data": [{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}],
				"model": "text-embedding-bge-m3",
				"usage": {"prompt_tokens": 1, "total_tokens": 1}
			}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(ClientDependency{
			BaseURL:        server.URL + "/v1",
			EmbeddingModel: "text-embedding-bge-m3",
		})

		vec, err := client.Embed(context.Background(), "hello")
		require.NoError(t, err)
		require.Equal(t, []float32{0.1, 0.2, 0.3}, vec)
		require.Equal(t, "/v1/embeddings", gotPath)
		require.Equal(t, "text-embedding-bge-m3", gotBody["model"])
		require.Equal(t, float64(EmbeddingDimensions), gotBody["dimensions"])
	})

	t.Run("return error when embedding endpoint returns no data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{
				"object": "list",
				"data": [],
				"model": "text-embedding-bge-m3",
				"usage": {"prompt_tokens": 0, "total_tokens": 0}
			}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(ClientDependency{BaseURL: server.URL + "/v1"})
		_, err := client.Embed(context.Background(), "hello")
		require.Error(t, err)
	})
}
