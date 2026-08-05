package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/auth"
	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/storage"
	"github.com/anditakaesar/uwa-go-rag/internal/role"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/user"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Database interface {
	Get() *pgxpool.Pool
	Close()
}

type StorageClient interface {
	Get() *storage.S3Client
}

type ServerDependency struct {
	DB            Database
	StorageClient StorageClient
}

type Executor struct {
	Mux         *chi.Mux
	RiverClient *river.Client[pgx.Tx]
}

func SetupServer(dep *ServerDependency) *Executor {
	router := chi.NewRouter()
	infraSvc := NewInfra(dep.DB.Get(), dep.StorageClient.Get())

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
			http.FileServer(http.Dir(env.Get().Values.UploadDir)),
		),
	)

	// handlers and routes
	mainHandler := web.NewMainHandler(web.MainHandlerDeps{
		UserService:   infraSvc.UserService,
		JWTService:    infraSvc.JWTService,
		JWTSecret:     env.Get().Values.JWTSecret,
		CookieService: infraSvc.CookieService,
		FileService:   infraSvc.FileService,
		WebRenderer:   infraSvc.WebRenderer,
	})

	userApi := user.NewUserApi(user.UserApiDeps{
		Service: infraSvc.UserService,
	})

	chatApi := chat.NewChatApi(chat.ChatApiDeps{
		ChatService: infraSvc.ChatService,
	})

	loginApi := auth.NewLoginApi(auth.LoginApiDeps{
		UserService:   infraSvc.UserService,
		JWTService:    infraSvc.JWTService,
		JWTSecret:     env.Get().Values.JWTSecret,
		CookieService: infraSvc.CookieService,
		AuditService:  infraSvc.AuditService,
	})

	roleApi := role.NewRoleApi(role.RoleApiDeps{
		RoleService: infraSvc.RoleService,
	})

	auditlogApi := audit.NewAuditLogApi(audit.ApiDependency{
		AuditLogService: infraSvc.AuditService,
	})

	fileApi := file.NewFileApi(file.FileApiDependency{
		FileService: infraSvc.FileService,
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

		web.SetupMainRoutes(r, mainHandler)
	})

	router.Route("/api", func(r chi.Router) {
		// middlewares
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   env.Get().CorsOptions.AllowedOrigins,
			AllowedMethods:   env.Get().CorsOptions.AllowedMethods,
			AllowedHeaders:   env.Get().CorsOptions.AllowedHeaders,
			ExposedHeaders:   env.Get().CorsOptions.ExposedHeaders,
			AllowCredentials: env.Get().CorsOptions.AllowCredentials,
			MaxAge:           env.Get().CorsOptions.MaxAge,
		}))
		r.Use(middlewares.GlobalErrorMiddleware)
		r.Use(middlewares.ResolveAuth(
			infraSvc.CookieService,
			infraSvc.UserService,
			infraSvc.JWTService,
		))
		r.Use(middlewares.ResolveUser(infraSvc.UserService))

		user.SetupUserApiRoutes(r, userApi)
		chat.SetupChatApiRoutes(r, chatApi)
		auth.SetupLoginApiRoutes(r, loginApi)
		role.SetupRoleApiRoutes(r, roleApi)
		audit.SetupAuditLogApiRoutes(r, auditlogApi)
		file.SetupFileApiRoutes(r, fileApi)
	})

	return &Executor{
		Mux:         router,
		RiverClient: infraSvc.RiverClient,
	}
}
