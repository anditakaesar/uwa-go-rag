package chat_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/chat/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendMessage_Success(t *testing.T) {
	svc := mocks.NewMockIChatService(t)

	chunkID := uuid.Must(uuid.NewV7())
	fileID := uuid.Must(uuid.NewV7())

	svc.EXPECT().Chat(mock.Anything, "Bagaimana cara setup autentikasi?").Return(&chat.ChatResponse{
		Message: "Autentikasi memerlukan Bearer token.",
		Citations: []chat.Citation{
			{
				ChunkID:     chunkID,
				FileID:      fileID,
				HeadingPath: []string{"# API Reference", "## Authentication"},
				Similarity:  0.92,
				Snippet:     "Bearer token required.",
			},
		},
	}, nil)

	api := chat.NewChatApi(chat.ChatApiDeps{ChatService: svc})
	h := handler.MakeHandler(api.SendMessage)

	body := strings.NewReader(`{"prompt":"Bagaimana cara setup autentikasi?"}`)
	req := httptest.NewRequest(http.MethodPost, "/chat/raw", body)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var envelope struct {
		Data chat.ChatResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&envelope))

	assert.Equal(t, "Autentikasi memerlukan Bearer token.", envelope.Data.Message)
	require.Len(t, envelope.Data.Citations, 1)
	assert.Equal(t, chunkID, envelope.Data.Citations[0].ChunkID)
	assert.Equal(t, fileID, envelope.Data.Citations[0].FileID)
	assert.Equal(t, []string{"# API Reference", "## Authentication"}, envelope.Data.Citations[0].HeadingPath)
	assert.InDelta(t, 0.92, envelope.Data.Citations[0].Similarity, 0.0001)
}

func TestSendMessage_EmptyPrompt(t *testing.T) {
	svc := mocks.NewMockIChatService(t)

	api := chat.NewChatApi(chat.ChatApiDeps{ChatService: svc})
	h := handler.MakeHandler(api.SendMessage)

	body := strings.NewReader(`{"prompt":"   "}`)
	req := httptest.NewRequest(http.MethodPost, "/chat/raw", body)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSendMessage_DecodingError(t *testing.T) {
	svc := mocks.NewMockIChatService(t)

	api := chat.NewChatApi(chat.ChatApiDeps{ChatService: svc})
	h := handler.MakeHandler(api.SendMessage)

	body := strings.NewReader(`{invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/chat/raw", body)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSendMessage_ChatError(t *testing.T) {
	svc := mocks.NewMockIChatService(t)
	svc.EXPECT().Chat(mock.Anything, "pertanyaan").Return(nil, errors.New("embed failed"))

	api := chat.NewChatApi(chat.ChatApiDeps{ChatService: svc})
	h := handler.MakeHandler(api.SendMessage)

	body := strings.NewReader(`{"prompt":"pertanyaan"}`)
	req := httptest.NewRequest(http.MethodPost, "/chat/raw", body)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
