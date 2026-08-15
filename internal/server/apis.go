package server

import (
	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/auth"
	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/role"
	"github.com/anditakaesar/uwa-go-rag/internal/user"
	"github.com/anditakaesar/uwa-go-rag/internal/web"
)

type Apis struct {
	MainHandler *web.MainHandler
	UserApi     *user.UserApi
	ChatApi     *chat.ChatApi
	LoginApi    *auth.AuthApi
	RoleApi     *role.RoleApi
	AuditLogApi *audit.Api
	FileApi     *file.FileApi
	FAQApi      *faq.FAQApi
}

func newApis(infraSvc *Services) *Apis {
	return &Apis{
		MainHandler: web.NewMainHandler(web.MainHandlerDeps{
			UserService:   infraSvc.UserService,
			JWTService:    infraSvc.JWTService,
			JWTSecret:     env.Get().Values.JWTSecret,
			CookieService: infraSvc.CookieService,
			FileService:   infraSvc.FileService,
			WebRenderer:   infraSvc.WebRenderer,
		}),
		UserApi: user.NewUserApi(user.UserApiDeps{
			Service: infraSvc.UserService,
		}),
		ChatApi: chat.NewChatApi(chat.ChatApiDeps{
			ChatService: infraSvc.ChatService,
		}),
		LoginApi: auth.NewLoginApi(auth.LoginApiDeps{
			UserService:   infraSvc.UserService,
			JWTService:    infraSvc.JWTService,
			JWTSecret:     env.Get().Values.JWTSecret,
			CookieService: infraSvc.CookieService,
			JobQueue:      infraSvc.JobQueue,
		}),
		RoleApi: role.NewRoleApi(role.RoleApiDeps{
			RoleService: infraSvc.RoleService,
		}),
		AuditLogApi: audit.NewAuditLogApi(audit.ApiDependency{
			AuditLogService: infraSvc.AuditService,
		}),
		FileApi: file.NewFileApi(file.FileApiDependency{
			FileService: infraSvc.FileService,
			JobQueue:    infraSvc.JobQueue,
			RagService:  infraSvc.RagService,
		}),
		FAQApi: faq.NewFAQApi(faq.FAQApiDependency{
			FAQService: infraSvc.FAQService,
		}),
	}
}
