package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

func TestSendJSON(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		// 1. Prepare dummy data
		type DummyData struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		data := DummyData{ID: 1, Name: "Test Object"}

		// 2. Setup recorder
		rr := httptest.NewRecorder()

		// 3. Call the function
		transport.SendJSON(rr, http.StatusCreated, data)

		// 4. Assertions
		// Check Status Code
		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rr.Code)
		}

		// Check Header
		expectedContentType := "application/json; charset=utf-8"
		if rr.Header().Get("Content-Type") != expectedContentType {
			t.Errorf("expected content type %s, got %s", expectedContentType, rr.Header().Get("Content-Type"))
		}

		// Check Body (Unmarshaling back to verify structure)
		var result struct {
			Data DummyData `json:"data"`
		}

		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}

		if result.Data.Name != "Test Object" {
			t.Errorf("expected name 'Test Object', got '%s'", result.Data.Name)
		}
	})
}

func TestMakeHandler(t *testing.T) {
	t.Run("success - handler returns nil", func(t *testing.T) {
		// Create a stub AppHandler that succeeds
		h := func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("success"))
			return nil
		}

		wrapped := handler.MakeHandler(h)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rr.Code)
		}
		if rr.Body.String() != "success" {
			t.Errorf("expected 'success', got %s", rr.Body.String())
		}
	})

	t.Run("failure - returns custom status code", func(t *testing.T) {
		// Assuming you have a custom error type that DefineStatusCode recognizes
		h := func(w http.ResponseWriter, r *http.Request) error {
			return &xerror.ErrorNotFound{} // this maps to 404
		}

		wrapped := handler.MakeHandler(h)
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for Not Found error, got %d", rr.Code)
		}
	})
}
