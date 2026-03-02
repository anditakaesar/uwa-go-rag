package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IDatabase interface {
	Get() *pgxpool.Pool
	Close()
}

type ServerDependency struct {
	DB IDatabase
}

func SetupServer(dep *ServerDependency) *chi.Mux {
	router := chi.NewRouter()
	infraSvc := infra.NewInfra(dep.DB.Get())

	// static files
	sub, err := fs.Sub(web.PublicFS, "public")
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("static file sub failed: %v", err))
		os.Exit(1)
	}

	router.Handle(
		"/static/*",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.FS(sub)),
		),
	)

	router.Handle(
		"/uploads/*",
		http.StripPrefix(
			"/uploads/",
			http.FileServer(http.Dir(env.Values.UploadDir)),
		),
	)

	// handlers and routes
	mainHandler := handler.NewMainHandler(handler.MainHandlerDeps{
		UserService:   infraSvc.UserService,
		JWTService:    infraSvc.JWTService,
		CookieService: infraSvc.CookieService,
		FileService:   infraSvc.FileService,
		WebRenderer:   infraSvc.WebRenderer,
	})

	userApi := handler.NewUserApi(handler.UserApiDeps{
		UserService: infraSvc.UserService,
	})

	router.Group(func(r chi.Router) {
		// middlewares
		r.Use(middlewares.GlobalErrorMiddleware)
		r.Use(middlewares.ResolveAuth(
			infraSvc.CookieService,
			infraSvc.UserService,
			infraSvc.JWTService,
		))
		r.Use(middlewares.ResolveUser(infraSvc.UserService))

		handler.SetupMainRoutes(r, mainHandler)
	})

	router.Route("/api", func(r chi.Router) {
		// middlewares
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"}, // Allow all origins
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           300, // Cache preflight response for 5 minutes
		}))
		r.Use(middlewares.GlobalErrorMiddleware)
		r.Use(middlewares.ResolveAuth(
			infraSvc.CookieService,
			infraSvc.UserService,
			infraSvc.JWTService,
		))
		r.Use(middlewares.ResolveUser(infraSvc.UserService))

		handler.SetupUserApiRoutes(r, userApi)
	})

	return router
}
