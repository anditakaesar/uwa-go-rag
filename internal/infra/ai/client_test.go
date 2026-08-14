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

func TestSendContextPrompt(t *testing.T) {
	t.Run("grounds answer in injected context with zero tools", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			err := json.NewDecoder(r.Body).Decode(&gotBody)
			require.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{
				"id": "resp_test",
				"object": "response",
				"output": [
					{
						"id": "msg_test",
						"type": "message",
						"role": "assistant",
						"status": "completed",
						"content": [
							{"type": "output_text", "text": "Autentikasi dijelaskan di bagian Authentication."}
						]
					}
				]
			}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(ClientDependency{BaseURL: server.URL + "/v1"})

		answer, err := client.SendContextPrompt(
			context.Background(),
			"[1] (# System Architecture > ## Authentication) similarity 0.9\nBearer token required.",
			"Bagaimana cara setup autentikasi?",
		)

		require.NoError(t, err)
		require.Equal(t, "Autentikasi dijelaskan di bagian Authentication.", answer)
		require.Equal(t, "/v1/responses", gotPath)
		require.Equal(t, "Bagaimana cara setup autentikasi?", gotBody["input"])
		require.Equal(t, chatModel, gotBody["model"])

		instructions, ok := gotBody["instructions"].(string)
		require.True(t, ok)
		require.Contains(t, instructions, "always answer in Bahasa Indonesia")
		require.Contains(t, instructions, "DILARANG menjawab dari pengetahuan umum")
		require.Contains(t, instructions, "===== KONTEKS =====")
		require.Contains(t, instructions, "[1] (# System Architecture > ## Authentication) similarity 0.9")
		require.Contains(t, instructions, "===== AKHIR KONTEKS =====")

		_, hasTools := gotBody["tools"]
		require.False(t, hasTools, "RAG call must register zero tools")
	})
}

func TestSendPrompt_legacy(t *testing.T) {
	t.Run("keeps base instructions without context", func(t *testing.T) {
		var gotBody map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewDecoder(r.Body).Decode(&gotBody)
			require.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{
				"id": "resp_test",
				"object": "response",
				"output": [
					{
						"id": "msg_test",
						"type": "message",
						"role": "assistant",
						"status": "completed",
						"content": [
							{"type": "output_text", "text": "halo"}
						]
					}
				]
			}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(ClientDependency{BaseURL: server.URL + "/v1"})

		_, err := client.SendPrompt(context.Background(), "halo")
		require.NoError(t, err)

		instructions, ok := gotBody["instructions"].(string)
		require.True(t, ok)
		require.Contains(t, instructions, "always answer in Bahasa Indonesia")
		require.NotContains(t, instructions, "===== KONTEKS =====")
	})
}

func TestSendPrompt_legacy_error(t *testing.T) {
	t.Run("returns error from responses endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClient(ClientDependency{BaseURL: server.URL + "/v1"})

		_, err := client.SendPrompt(context.Background(), "halo")
		require.Error(t, err)
	})
}
