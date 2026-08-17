package faq_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/faq/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errQuery = errors.New("query_error")

func idPtr(v int64) *int64 {
	return &v
}

func contextWithIdentity(r *http.Request, userID int64) context.Context {
	return context.WithValue(r.Context(), domain.IdentityKey, domain.Identity{UserID: userID})
}

func TestFAQApi_List(test *testing.T) {
	test.Parallel()

	test.Run("success - returns unanswered FAQs with pagination", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		now := time.Now().UTC()
		faqs := []domain.FAQ{
			{ID: uuid.Must(uuid.NewV7()), Question: "Bagaimana cara reset password?", Status: domain.FAQStatusUnanswered, CreatedAt: now},
		}
		svc.EXPECT().List(mock.Anything, domain.FAQStatusUnanswered, 10, 0).Return(faqs, nil)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		h := handler.MakeHandler(api.List)

		req := httptest.NewRequest(http.MethodGet, "/faqs", nil)
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var envelope struct {
			Data []faq.FAQListResponse `json:"data"`
			Meta struct {
				Page  int   `json:"page"`
				Size  int   `json:"size"`
				Total int64 `json:"total"`
			} `json:"meta"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&envelope))

		require.Len(t, envelope.Data, 1)
		assert.Equal(t, "Bagaimana cara reset password?", envelope.Data[0].Question)
		assert.Equal(t, 1, envelope.Meta.Page)
		assert.Equal(t, 10, envelope.Meta.Size)
	})

	test.Run("status filter - answered passed through", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		answered := []domain.FAQ{
			{ID: uuid.Must(uuid.NewV7()), Question: "q?", Status: domain.FAQStatusAnswered},
		}
		svc.EXPECT().List(mock.Anything, domain.FAQStatusAnswered, 10, 0).Return(answered, nil)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		h := handler.MakeHandler(api.List)

		req := httptest.NewRequest(http.MethodGet, "/faqs?status=answered", nil)
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
	})

	test.Run("invalid status - validation error", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		h := handler.MakeHandler(api.List)

		req := httptest.NewRequest(http.MethodGet, "/faqs?status=bogus", nil)
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	test.Run("service failure - propagates", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)
		svc.EXPECT().List(mock.Anything, domain.FAQStatusUnanswered, 10, 0).Return([]domain.FAQ{}, errQuery)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		h := handler.MakeHandler(api.List)

		req := httptest.NewRequest(http.MethodGet, "/faqs", nil)
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestFAQApi_Answer(test *testing.T) {
	test.Parallel()

	faqID := uuid.Must(uuid.NewV7())

	test.Run("success - persists answer and returns the FAQ", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		answeredAt := time.Now().UTC()
		answered := &domain.FAQ{
			ID:         faqID,
			Question:   "Bagaimana cara reset password?",
			Answer:     "Buka halaman Login, klik Lupa Password.",
			Status:     domain.FAQStatusAnswered,
			AnsweredBy: idPtr(int64(42)),
			AnsweredAt: &answeredAt,
		}
		svc.EXPECT().Answer(mock.Anything, faqID, "Buka halaman Login, klik Lupa Password.", int64(42)).Return(answered, nil)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		router := chi.NewRouter()
		router.Method(http.MethodPut, "/faqs/{uuid}/answer", handler.MakeHandler(api.Answer))

		body := strings.NewReader(`{"answer":"Buka halaman Login, klik Lupa Password."}`)
		req := httptest.NewRequest(http.MethodPut, "/faqs/"+faqID.String()+"/answer", body)
		req = req.WithContext(contextWithIdentity(req, 42))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var envelope struct {
			Data faq.FAQAnswerResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&envelope))

		assert.Equal(t, faqID, envelope.Data.ID)
		assert.Equal(t, "answered", envelope.Data.Status)
		require.NotNil(t, envelope.Data.AnsweredBy)
		assert.Equal(t, int64(42), *envelope.Data.AnsweredBy)
	})

	test.Run("invalid uuid - not found", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		router := chi.NewRouter()
		router.Method(http.MethodPut, "/faqs/{uuid}/answer", handler.MakeHandler(api.Answer))

		body := strings.NewReader(`{"answer":"answer"}`)
		req := httptest.NewRequest(http.MethodPut, "/faqs/not-a-uuid/answer", body)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	test.Run("decode error - bad request", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		router := chi.NewRouter()
		router.Method(http.MethodPut, "/faqs/{uuid}/answer", handler.MakeHandler(api.Answer))

		body := strings.NewReader(`{invalid`)
		req := httptest.NewRequest(http.MethodPut, "/faqs/"+faqID.String()+"/answer", body)
		req = req.WithContext(contextWithIdentity(req, 42))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestFAQApi_Delete(test *testing.T) {
	test.Parallel()

	faqID := uuid.Must(uuid.NewV7())

	test.Run("success", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)
		svc.EXPECT().Delete(mock.Anything, faqID).Return(nil)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		router := chi.NewRouter()
		router.Method(http.MethodDelete, "/faqs/{uuid}", handler.MakeHandler(api.Delete))

		req := httptest.NewRequest(http.MethodDelete, "/faqs/"+faqID.String(), nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	test.Run("invalid uuid - not found", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		router := chi.NewRouter()
		router.Method(http.MethodDelete, "/faqs/{uuid}", handler.MakeHandler(api.Delete))

		req := httptest.NewRequest(http.MethodDelete, "/faqs/not-a-uuid", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	test.Run("service failure - propagates", func(t *testing.T) {
		svc := mocks.NewMockFAQService(t)
		svc.EXPECT().Delete(mock.Anything, faqID).Return(errQuery)

		api := faq.NewFAQApi(faq.ApiDependency{FAQService: svc})
		router := chi.NewRouter()
		router.Method(http.MethodDelete, "/faqs/{uuid}", handler.MakeHandler(api.Delete))

		req := httptest.NewRequest(http.MethodDelete, "/faqs/"+faqID.String(), nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
