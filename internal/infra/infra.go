package infra

import (
	"fmt"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/repo"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Services struct {
	UserService   *service.UserService
	JWTService    *JWTService
	CookieService *CookieSvc
	FileService   *service.FileService
	WebRenderer   *web.Renderer
	ChatService   *service.ChatService
	RiverClient   *river.Client[pgx.Tx]
}

func NewInfra(pool *pgxpool.Pool) *Services {
	userRepo := repo.NewUserRepository(pool)
	uow := NewUnitOfWork(pool)
	riverQueue := NewRiverQueue()
	aiClient := NewAIClient(AIClientDep{
		BaseURL: env.Values.AIBaseURL,
		ApiKey:  env.Values.AIAPIKey,
	})

	userSvc := service.NewUserService(service.UserServiceDeps{
		UserRepo:    userRepo,
		PassChecker: NewPasswordHelper(env.Values.PassSecret),
		UOW:         uow,
	})
	jwtSvc := NewJWTService(env.Values.JWTSecret)
	cookieService := NewCookieService(env.Values.IsDevelopment(), env.Values.CookieSecret)
	fileSvc := service.NewFileService(env.Values.UploadDir, env.UPLOAD_ALLOWED_TYPES)
	chatSvc := service.NewChatService(aiClient, riverQueue)

	// queue workers
	workers, err := worker.RegisterWorkers(worker.RegisterWorkerDep{
		ChatService: chatSvc,
	})
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("error setup worker client: %v", err))
		os.Exit(1)
	}

	var riverClient *river.Client[pgx.Tx]
	if pool != nil {
		riverClient, err = NewRiverClient(pool, workers)
		if err != nil {
			xlog.Logger.Error(fmt.Sprintf("error setup worker client: %v", err))
			os.Exit(1)
		}
		riverQueue.SetClient(riverClient)
	}

	return &Services{
		UserService:   userSvc,
		JWTService:    jwtSvc,
		CookieService: cookieService,
		FileService:   fileSvc,
		WebRenderer:   web.NewRenderer(),
		ChatService:   chatSvc,
		RiverClient:   riverClient,
	}
}
