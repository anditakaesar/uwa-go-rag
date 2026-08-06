package server

import (
	"fmt"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/ai"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/cookie"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/db/postgres"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/jwt"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/password"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/queue"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/storage"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
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
	UserService   *user.Service
	JWTService    *jwt.Service
	CookieService *cookie.Service
	FileService   *file.Service
	WebRenderer   *web.Renderer
	ChatService   *chat.ChatService
	RoleService   *role.RoleService
	RiverClient   *river.Client[pgx.Tx]
	AuditService  *audit.AuditRecorder
	StorageClient *storage.RustFS
	JobQueue      *queue.RiverQueue
}

func NewInfra(pool *pgxpool.Pool, infraStorage *storage.S3Client) *Services {
	userRepo := postgres.NewUserRepository(pool)
	ragRepo := postgres.NewRagRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	roleRepo := postgres.NewRoleRepository(pool)
	rolePermissionRepo := postgres.NewPermissionRepo(pool)
	fileRepo := postgres.NewFileRepository(pool)
	uow := postgres.NewUnitOfWork(pool)
	riverQueue := queue.NewRiverQueue()

	storageClient := storage.NewRustFs(storage.RustFSDependency{
		StorageClient: infraStorage,
		BucketName:    env.Get().S3Config.S3Bucket,
		BucketPrefix:  env.Get().S3Config.S3Prefix,
	})

	aiClient := ai.NewClient(ai.ClientDependency{
		BaseURL: env.Get().Values.AIBaseURL,
		ApiKey:  env.Get().Values.AIAPIKey,
	})

	userSvc := user.NewUserService(user.ServiceDependency{
		UserRepo:    userRepo,
		PassChecker: password.NewArgonClientHelper(env.Get().Values.PassSecret),
		UOW:         uow,
	})

	jwtSvc := jwt.NewJWTService(jwt.ServiceDependency{
		Secret:             []byte(env.Get().Values.JWTSecret),
		JWTExpire:          env.Get().Values.JWTExpire,
		RolePermissionRepo: rolePermissionRepo,
	})

	cookieService := cookie.NewService(env.Get().Values.IsDevelopment(), env.Get().Values.CookieSecret)

	fileSvc := file.NewService(file.ServiceDependency{
		DirName:       env.Get().Values.UploadDir,
		AllowedTypes:  env.UPLOAD_ALLOWED_TYPES,
		StorageClient: storageClient,
		FileRepo:      fileRepo,
		UOW:           uow,
	})

	chatSvc := chat.NewChatService(chat.ChatServiceDep{
		RagRepo:   ragRepo,
		AIClient:  aiClient,
		JobQueue:  riverQueue,
		UploadDir: env.Get().Values.UploadDir,
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
		Recorder:    auditSvc,
	})
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("error setup worker client: %v", err))
		os.Exit(1)
	}

	var riverClient *river.Client[pgx.Tx]
	if pool != nil {
		riverClient, err = queue.NewRiverClient(pool, workers)
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
		JobQueue:      riverQueue,
	}
}
