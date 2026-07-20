package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/handler"
	"github.com/go-chi/chi/v5"
)

func TestSetupUserApiRoutes(test *testing.T) {

	test.Run("setup user routes", func(t *testing.T) {
		h := &handler.UserApi{}
		r := chi.NewRouter()

		handler.SetupUserApiRoutes(r, h)

		expectedRoutes := []struct {
			method string
			path   string
		}{
			{http.MethodPost, "/users"},
			{http.MethodGet, "/users"},
		}

		foundRoutes := make(map[string]bool)

		walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			key := fmt.Sprintf("%s %s", method, route)
			foundRoutes[key] = true
			return nil
		}

		if err := chi.Walk(r, walkFunc); err != nil {
			t.Fatalf("Failed to walk router: %v", err)
		}

		for _, exp := range expectedRoutes {
			key := fmt.Sprintf("%s %s", exp.method, exp.path)
			if !foundRoutes[key] {
				t.Errorf("Route [%s] was not registered", key)
			}
		}
	})
}
