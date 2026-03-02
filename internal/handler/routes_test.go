package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/handler"
	"github.com/go-chi/chi/v5"
)

func TestSetupMainRoutes(test *testing.T) {
	test.Run("setup routes registration", func(t *testing.T) {
		// 1. Setup a dummy handler (doesn't need real services)
		h := &handler.MainHandler{}
		r := chi.NewRouter()

		// 2. Call your setup function
		handler.SetupMainRoutes(r, h)

		// 3. Define what we EXPECT to find
		expectedRoutes := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/"},
			{http.MethodGet, "/login"},
			{http.MethodPost, "/login"},
			{http.MethodGet, "/logout"},
			{http.MethodGet, "/upload"},
			{http.MethodPost, "/upload"},
		}

		// 4. Create a map to track what chi actually registered
		foundRoutes := make(map[string]bool)

		walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			// chi stores the full path; we format it as "METHOD /path"
			key := fmt.Sprintf("%s %s", method, route)
			foundRoutes[key] = true
			return nil
		}

		if err := chi.Walk(r, walkFunc); err != nil {
			t.Fatalf("Failed to walk router: %v", err)
		}

		// 5. Assertions
		for _, exp := range expectedRoutes {
			key := fmt.Sprintf("%s %s", exp.method, exp.path)
			if !foundRoutes[key] {
				t.Errorf("Route [%s] was not registered", key)
			}
		}
	})
}

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
