package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
)

type CookieService interface {
	Get(r *http.Request, name string) (*sessions.Session, error)
	Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error
}

type JWTService interface {
	Verify(token string) (domain.UserClaims, error)
	IssueJWT(ctx context.Context, userID int64, secret []byte) (string, error)

	VerifyRefreshToken(ctx context.Context, token string) (domain.RefreshTokenClaims, error)
	IssueRefreshToken(ctx context.Context, param common.RefreshTokenParam) (string, error)
}

type UserService interface {
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
}

type Middleware func(http.Handler) http.Handler

func CSRFMiddleware() Middleware {
	secure := !env.Get().Values.IsDevelopment()

	opts := []csrf.Option{
		csrf.FieldName(env.CSRF_TOKEN_FIELD_NAME),
		csrf.Secure(secure),
	}

	if !secure {
		opts = append(opts,
			csrf.TrustedOrigins([]string{
				"localhost" + env.Get().Values.Port,
			}),
		)
	}

	return csrf.Protect(
		[]byte(env.Get().Values.CSRFSecret),
		opts...,
	)
}

func ResolveAuth(
	cookieStore CookieService,
	userService UserService,
	jwtService JWTService,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			tokenStr, found := strings.CutPrefix(auth, "Bearer ")
			if found {
				claims, err := jwtService.Verify(tokenStr)
				if err == nil {
					userID, _ := strconv.ParseInt(claims.Subject, 10, 64)
					ctx := context.WithValue(
						r.Context(),
						domain.IdentityKey,
						domain.Identity{UserID: userID, Permission: claims.Permissions, Method: "jwt"},
					)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			session, err := cookieStore.Get(r, "auth_session")
			if err == nil {
				uid, ok := session.Values["user_id"].(int64)
				if ok {
					ctx := context.WithValue(
						r.Context(),
						domain.IdentityKey,
						domain.Identity{UserID: uid, Method: "session"},
					)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ResolveUser(userService UserService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := r.Context().Value(domain.IdentityKey).(domain.Identity)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			user, _ := userService.GetUserByID(r.Context(), identity.UserID)
			if user != nil {
				ctx := context.WithValue(
					r.Context(),
					domain.UserCtxKey,
					user,
				)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequirePermission(permission string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := domain.IdentityFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if !slices.Contains(user.Permission, permission) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := r.Context().Value(domain.IdentityKey).(domain.Identity)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GlobalErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				xlog.Logger.Error(fmt.Sprintf("PANIC RECOVERED: %v", rvr))

				transport.SendError(w, http.StatusInternalServerError,
					transport.ErrObj{
						Title:   "Internal Server Error",
						Message: "An unexpected error happened.",
					})
			}
		}()

		next.ServeHTTP(w, r)
	})
}
