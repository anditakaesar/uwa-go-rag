package transport_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/stretchr/testify/assert"
)

func TestSendJSON(test *testing.T) {

	test.Run("success return data", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "hello"}

		transport.SendJSON(w, http.StatusOK, data)

		// Check Status
		assert.Equal(test, http.StatusOK, w.Code)

		// Check Content-Type
		assert.Equal(test, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

		// Check Body
		expected := `{"data":{"message":"hello"}}`
		assert.JSONEq(test, expected, w.Body.String())
	})

	test.Run("success return data and meta", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "hello"}
		meta := map[string]string{"metamsg": "message-meta"}

		transport.SendJSON(w, http.StatusOK, data, transport.WithMeta(meta))

		// Check Status
		assert.Equal(test, http.StatusOK, w.Code)

		// Check Content-Type
		assert.Equal(test, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

		// Check Body
		expected := `{"data":{"message":"hello"}, "meta":{"metamsg":"message-meta"}}`
		assert.JSONEq(test, expected, w.Body.String())
	})

	test.Run("fails to encode complex type", func(t *testing.T) {
		w := httptest.NewRecorder()

		// Channels cannot be encoded to JSON
		badData := make(chan int)

		transport.SendJSON(w, http.StatusOK, badData)

		// It should catch the error and return 500 instead of 200
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal server error")
	})

}

func TestSendError(t *testing.T) {
	t.Run("standard error response", func(t *testing.T) {
		w := httptest.NewRecorder()
		errObj := transport.ErrObj{
			Title:   "Unauthorized",
			Message: "Invalid token provided",
		}

		transport.SendError(w, http.StatusUnauthorized, errObj)

		// Assertions
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

		var resp transport.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		assert.Equal(t, "Unauthorized", resp.Error.Title)
		assert.Equal(t, "Invalid token provided", resp.Error.Message)
	})

	t.Run("error with missing fields", func(t *testing.T) {
		w := httptest.NewRecorder()
		errObj := transport.ErrObj{Message: "Generic error"}

		transport.SendError(w, http.StatusBadRequest, errObj)

		// The omitempty tag in your struct should remove Title from the JSON
		assert.NotContains(t, w.Body.String(), "title")
		assert.Contains(t, w.Body.String(), `"message":"Generic error"`)
	})
}
