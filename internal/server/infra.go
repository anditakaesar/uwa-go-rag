package server

import (
	"fmt"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/jwt"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/anditakaesar/uwa-go-rag/internal/repo"
	"github.com/anditakaesar/uwa-go-rag/internal/role"
	"github.com/anditakaesar/uwa-go-rag/internal/user"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Services struct {
	UserService   *user.UserService
	JWTService    *jwt.JWTService
	CookieService *infra.CookieSvc
	FileService   *file.FileService
	WebRenderer   *web.Renderer
	ChatService   *chat.ChatService
	RoleService   *role.RoleService
	RiverClient   *river.Client[pgx.Tx]
	AuditService  *audit.AuditRecorder
	StorageClient *infra.RustFS
}

func NewInfra(pool *pgxpool.Pool, infraStorage *infra.InfraStorageClient) *Services {
	userRepo := repo.NewUserRepository(pool)
	ragRepo := rag.NewRagRepository(pool)
	auditRepo := audit.NewAuditRepository(pool)
	roleRepo := repo.NewRoleRepository(pool)
	rolePermissionRepo := repo.NewRolePermissionRepo(pool)
	fileRepo := file.NewFileRepo(pool)
	uow := infra.NewUnitOfWork(pool)
	riverQueue := infra.NewRiverQueue()
	storageClient := infra.NewRustFs(infra.RustFSDependency{
		StorageClient: infraStorage,
		BucketName:    env.S3Conf.S3Bucket,
		BucketPrefix:  env.S3Conf.S3Prefix,
	})
	aiClient := infra.NewAIClient(infra.AIClientDep{
		BaseURL: env.Values.AIBaseURL,
		ApiKey:  env.Values.AIAPIKey,
	})

	userSvc := user.NewUserService(user.UserServiceDeps{
		UserRepo:    userRepo,
		PassChecker: infra.NewPasswordHelper(env.Values.PassSecret),
		UOW:         uow,
	})
	jwtSvc := jwt.NewJWTService(jwt.JWTServiceDep{
		Secret:             []byte(env.Values.JWTSecret),
		JWTExpire:          env.Values.JWTExpire,
		RolePermissionRepo: rolePermissionRepo,
	})
	cookieService := infra.NewCookieService(env.Values.IsDevelopment(), env.Values.CookieSecret)
	fileSvc := file.NewFileService(file.FileServiceDep{
		DirName:       env.Values.UploadDir,
		AllowedTypes:  env.UPLOAD_ALLOWED_TYPES,
		StorageClient: storageClient,
		FileRepo:      fileRepo,
		UOW:           uow,
	})
	chatSvc := chat.NewChatService(chat.ChatServiceDep{
		RagRepo:   ragRepo,
		AIClient:  aiClient,
		JobQueue:  riverQueue,
		UploadDir: env.Values.UploadDir,
	})
	ragSvc := rag.NewRagService()
	auditSvc := audit.NewAuditLogRecorder(auditRepo)
	roleSvc := role.NewRoleService(role.RoleServiceDep{
		RoleRepo: roleRepo,
	})

	// queue workers
	workers, err := worker.RegisterWorkers(worker.RegisterWorkerDep{
		ChatService: chatSvc,
		RagService:  ragSvc,
	})
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("error setup worker client: %v", err))
		os.Exit(1)
	}

	var riverClient *river.Client[pgx.Tx]
	if pool != nil {
		riverClient, err = infra.NewRiverClient(pool, workers)
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
		RoleService:   roleSvc,
		RiverClient:   riverClient,
		AuditService:  auditSvc,
		StorageClient: storageClient,
	}
}
