package infra

import (
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/repo"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Services struct {
	UserService   *service.UserService
	JWTService    *JWTService
	CookieService *CookieSvc
	FileService   *service.FileService
	WebRenderer   *web.Renderer
}

func NewInfra(pool *pgxpool.Pool) *Services {
	userRepo := repo.NewUserRepository(pool)
	uow := NewUnitOfWork(pool)
	userSvc := service.NewUserService(service.UserServiceDeps{
		UserRepo:    userRepo,
		PassChecker: NewPasswordHelper(env.Values.PassSecret),
		UOW:         uow,
	})
	jwtSvc := NewJWTService(env.Values.JWTSecret)
	cookieService := NewCookieService(env.Values.IsDevelopment(), env.Values.CookieSecret)
	fileSvc := service.NewFileService(env.Values.UploadDir, env.UPLOAD_ALLOWED_TYPES)

	return &Services{
		UserService:   userSvc,
		JWTService:    jwtSvc,
		CookieService: cookieService,
		FileService:   fileSvc,
		WebRenderer:   web.NewRenderer(),
	}
}
