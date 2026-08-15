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
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/role"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/user"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func registerStaticRoutes(r chi.Router) {
	sub, err := fs.Sub(web.PublicFS, "public")
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("static file sub failed: %v", err))
		os.Exit(1)
	}

	r.Handle(
		"/static/*",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.FS(sub)),
		),
	)

	r.Handle(
		"/uploads/*",
		http.StripPrefix(
			"/uploads/",
			http.FileServer(http.Dir(env.Get().Values.UploadDir)),
		),
	)
}

func registerMainRoutes(r chi.Router, infraSvc *Services, apis *Apis) {
	r.Use(middlewares.GlobalErrorMiddleware)
	r.Use(middlewares.ResolveAuth(
		infraSvc.CookieService,
		infraSvc.UserService,
		infraSvc.JWTService,
	))
	r.Use(middlewares.ResolveUser(infraSvc.UserService))

	web.SetupMainRoutes(r, apis.MainHandler)
}

func registerAPIRoutes(r chi.Router, infraSvc *Services, apis *Apis) {
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

	user.SetupUserApiRoutes(r, apis.UserApi)
	chat.SetupChatApiRoutes(r, apis.ChatApi)
	auth.SetupLoginApiRoutes(r, apis.LoginApi)
	role.SetupRoleApiRoutes(r, apis.RoleApi)
	audit.SetupAuditLogApiRoutes(r, apis.AuditLogApi)
	file.SetupFileApiRoutes(r, apis.FileApi)
	faq.SetupFAQApiRoutes(r, apis.FAQApi)
}
