package infra

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret             []byte
	jwtExpire          int
	rolePermissionRepo IInfraRolePermissionRepo
}

type JWTServiceDep struct {
	Secret             []byte
	JWTExpire          int
	RolePermissionRepo IInfraRolePermissionRepo
}

func NewJWTService(dep JWTServiceDep) *JWTService {
	return &JWTService{
		secret:             dep.Secret,
		jwtExpire:          dep.JWTExpire,
		rolePermissionRepo: dep.RolePermissionRepo,
	}
}

func (s *JWTService) Verify(token string) (domain.UserClaims, error) {
	claims := &domain.UserClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return domain.UserClaims{}, errors.New("invalid token")
	}

	return *claims, nil
}

func (s *JWTService) IssueJWT(ctx context.Context, userID int64, secret []byte) (string, error) {
	permissions, err := s.rolePermissionRepo.GetPermissionsByUser(ctx, userID)
	if err != nil {
		return "", err
	}

	claims := domain.UserClaims{
		Permissions: domain.ListPermissionName(permissions),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(
				time.Now().Add(time.Duration(s.jwtExpire) * time.Minute), // accessToken should be shortlived
			),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func (s *JWTService) IssueRefreshToken(ctx context.Context, param common.RefreshTokenParam) (string, error) {
	claims := domain.RefreshTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: strconv.FormatInt(param.UserID, 10),
			ExpiresAt: jwt.NewNumericDate(
				time.Now().Add(time.Duration(param.MaxAgeExpiration) * time.Second),
			),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(param.Secret)
}

func (s *JWTService) VerifyRefreshToken(ctx context.Context, token string) (domain.RefreshTokenClaims, error) {
	claims := &domain.RefreshTokenClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return domain.RefreshTokenClaims{}, errors.New("invalid refresh token")
	}

	return *claims, nil
}
