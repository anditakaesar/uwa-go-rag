package infra

import (
	"errors"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret []byte
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret: []byte(secret),
	}
}

func (s *JWTService) Verify(token string) (domain.UserClaims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return domain.UserClaims{}, errors.New("invalid token")
	}

	claims := parsed.Claims.(jwt.MapClaims)

	userID := int64(claims["sub"].(float64))
	exp := time.Unix(int64(claims["exp"].(float64)), 0)

	return domain.UserClaims{UserID: int64(userID), Exp: exp}, nil
}

func (s *JWTService) IssueJWT(userID int64, secret []byte) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
