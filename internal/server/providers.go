package server

import (
	"fmt"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/application"
	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/ai"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/cookie"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/db/postgres"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/jwt"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/password"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/queue"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/storage"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/anditakaesar/uwa-go-rag/internal/role"
	"github.com/anditakaesar/uwa-go-rag/internal/user"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type repositorySet struct {
	userRepo           *postgres.UserRepository
	chunkRepo          *postgres.ChunkRepository
	auditRepo          *postgres.AuditRepository
	roleRepo           *postgres.RoleRepository
	rolePermissionRepo *postgres.PermissionRepo
	fileRepo           *postgres.FileRepository
	faqRepo            *postgres.FaqRepository
	uow                application.UnitOfWork
}

func newRepositorySet(pool *pgxpool.Pool) *repositorySet {
	repos := &repositorySet{
		userRepo:           postgres.NewUserRepository(pool),
		chunkRepo:          postgres.NewChunkRepository(pool),
		auditRepo:          postgres.NewAuditRepository(pool),
		roleRepo:           postgres.NewRoleRepository(pool),
		rolePermissionRepo: postgres.NewPermissionRepo(pool),
		fileRepo:           postgres.NewFileRepository(pool),
		uow:                postgres.NewUnitOfWork(pool),
	}
	repos.faqRepo = postgres.NewFaqRepository(pool, repos.fileRepo)

	return repos
}

type clientSet struct {
	storageClient *storage.RustFS
	aiClient      *ai.AIClient
	riverQueue    *queue.RiverQueue
	tokenizer     *tokenization.SimpleTokenizer
}

func newClientSet(infraStorage *storage.S3Client) *clientSet {
	return &clientSet{
		storageClient: storage.NewRustFs(storage.RustFSDependency{
			StorageClient: infraStorage,
			BucketName:    env.Get().S3Config.S3Bucket,
			BucketPrefix:  env.Get().S3Config.S3Prefix,
		}),
		aiClient: ai.NewClient(ai.ClientDependency{
			BaseURL:        env.Get().Values.AIBaseURL,
			ApiKey:         env.Get().Values.AIAPIKey,
			EmbeddingModel: env.Get().Values.AIEmbeddingModel,
		}),
		riverQueue: queue.NewRiverQueue(),
		tokenizer:  tokenization.NewSimpleTokenizer(),
	}
}

type serviceSet struct {
	userSvc   *user.Service
	jwtSvc    *jwt.Service
	cookieSvc *cookie.Service
	fileSvc   *file.Service
	chatSvc   *chat.Service
	ragSvc    *rag.Service
	auditSvc  *audit.AuditRecorder
	roleSvc   *role.RoleService
	faqSvc    *faq.Service
}

func newServiceSet(repos *repositorySet, clients *clientSet) *serviceSet {
	faqSvc := faq.NewService(faq.ServiceDependency{
		Repo:     repos.faqRepo,
		UOW:      repos.uow,
		JobQueue: clients.riverQueue,
	})

	return &serviceSet{
		userSvc: user.NewUserService(user.ServiceDependency{
			UserRepo:    repos.userRepo,
			PassChecker: password.NewArgonClientHelper(env.Get().Values.PassSecret),
			UOW:         repos.uow,
		}),
		jwtSvc: jwt.NewJWTService(jwt.ServiceDependency{
			Secret:             []byte(env.Get().Values.JWTSecret),
			JWTExpire:          env.Get().Values.JWTExpire,
			RolePermissionRepo: repos.rolePermissionRepo,
		}),
		cookieSvc: cookie.NewService(env.Get().Values.IsDevelopment(), env.Get().Values.CookieSecret),
		fileSvc: file.NewService(file.ServiceDependency{
			DirName:       env.Get().Values.UploadDir,
			AllowedTypes:  env.UPLOAD_ALLOWED_TYPES,
			StorageClient: clients.storageClient,
			FileRepo:      repos.fileRepo,
			UOW:           repos.uow,
			JobQueue:      clients.riverQueue,
		}),
		chatSvc: chat.NewService(chat.ServiceDependency{
			ChunkRepo: repos.chunkRepo,
			AIClient:  clients.aiClient,
			Embedder:  clients.aiClient,
			Recorder:  faqSvc,
			Queue:     clients.riverQueue,
			UploadDir: env.Get().Values.UploadDir,
		}),
		ragSvc: rag.NewRagService(rag.ServiceDependency{
			Tokenizer: clients.tokenizer,
			JobQueue:  clients.riverQueue,
			FileRepo:  repos.fileRepo,
		}),
		auditSvc: audit.NewAuditLogRecorder(repos.auditRepo),
		roleSvc: role.NewRoleService(role.RoleServiceDep{
			RoleRepo: repos.roleRepo,
		}),
		faqSvc: faqSvc,
	}
}

func registerRiver(pool *pgxpool.Pool, repos *repositorySet, svcs *serviceSet, clients *clientSet) (*river.Client[pgx.Tx], error) {
	workers, err := worker.RegisterWorkers(worker.RegisterWorkerDep{
		RagService:      svcs.ragSvc,
		ChunkRepository: repos.chunkRepo,
		Embedder:        clients.aiClient,
		Recorder:        svcs.auditSvc,
		FileService:     svcs.fileSvc,
		StorageClient:   clients.storageClient,
		JobQueue:        clients.riverQueue,
		FaqRepository:   repos.faqRepo,
	})
	if err != nil {
		return nil, err
	}

	if pool == nil {
		return nil, nil
	}

	riverClient, err := queue.NewRiverClient(pool, workers)
	if err != nil {
		return nil, err
	}
	clients.riverQueue.SetClient(riverClient)

	return riverClient, nil
}

func mustRegisterRiver(pool *pgxpool.Pool, repos *repositorySet, svcs *serviceSet, clients *clientSet) *river.Client[pgx.Tx] {
	riverClient, err := registerRiver(pool, repos, svcs, clients)
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("error setup worker client: %v", err))
		os.Exit(1)
	}
	return riverClient
}
