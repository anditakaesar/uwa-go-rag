package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/gorilla/csrf"
)

type Middleware func(http.Handler) http.Handler

func CSRFMiddleware() Middleware {
	secure := !env.Values.IsDevelopment()

	opts := []csrf.Option{
		csrf.FieldName(env.CSRF_TOKEN_FIELD_NAME),
		csrf.Secure(secure),
	}

	if !secure {
		opts = append(opts,
			csrf.TrustedOrigins([]string{
				"localhost" + env.Values.Port,
			}),
		)
	}

	return csrf.Protect(
		[]byte(env.Values.CSRFSecret),
		opts...,
	)
}

func ResolveAuth(
	cookieStore infra.ICookieService,
	userService service.IUserService,
	jwtService infra.IJWTService,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			next.ServeHTTP(w, r)
		})
	}
}

func ResolveUser(userService service.IUserService) Middleware {
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
