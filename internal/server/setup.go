package server

import (
	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/cookie"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/jwt"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/queue"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/storage"
	"github.com/anditakaesar/uwa-go-rag/internal/role"
	"github.com/anditakaesar/uwa-go-rag/internal/user"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
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
	repos := newRepositorySet(pool)
	clients := newClientSet(infraStorage)
	svcs := newServiceSet(repos, clients)
	riverClient := mustRegisterRiver(pool, svcs, clients)

	return &Services{
		UserService:   svcs.userSvc,
		JWTService:    svcs.jwtSvc,
		CookieService: svcs.cookieSvc,
		FileService:   svcs.fileSvc,
		WebRenderer:   web.NewRenderer(),
		ChatService:   svcs.chatSvc,
		RoleService:   svcs.roleSvc,
		RiverClient:   riverClient,
		AuditService:  svcs.auditSvc,
		StorageClient: clients.storageClient,
		JobQueue:      clients.riverQueue,
	}
}
