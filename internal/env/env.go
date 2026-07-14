package env

import (
	"os"
	"strconv"
)

var Values *Object

type Object struct {
	Env          string
	Port         string
	DBUrl        string
	CookieSecret string
	CSRFSecret   string
	JWTSecret    string
	JWTExpire    int
	PassSecret   string
	UploadDir    string
	HostName     string
	AIBaseURL    string
	AIAPIKey     string
}

func Load() {
	Values = &Object{
		Env:          os.Getenv("ENV"),
		Port:         os.Getenv("PORT"),
		DBUrl:        os.Getenv("DB_URL"),
		CookieSecret: os.Getenv("COOKIE_SECRET"),
		CSRFSecret:   os.Getenv("CSRF_SECRET"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		JWTExpire:    getJWTExpireSession(),
		PassSecret:   os.Getenv("PASS_SECRET"),
		UploadDir:    os.Getenv("UPLOAD_DIR"),
		HostName:     os.Getenv("HOSTNAME"),
		AIBaseURL:    os.Getenv("AI_BASE_URL"),
		AIAPIKey:     os.Getenv("AI_API_KEY"),
	}
}

func getJWTExpireSession() int {
	str := os.Getenv("JWT_EXPIRE")
	value, err := strconv.Atoi(str)
	if err != nil || value < 10 {
		return 15
	}
	return value
}

func (v *Object) IsDevelopment() bool {
	return v.Env == "dev" || v.Env == "development"
}

const (
	CSRF_TOKEN_FIELD_NAME = "csrf_token"
	IDENTITY_KEY          = "identity-key"
	USER_CTX_KEY          = "registered_user_ctx"
	MAX_UPLOAD_SIZE       = 10 * 1024 * 1024 // 10 MB limit
)

var (
	UPLOAD_ALLOWED_TYPES = map[string]bool{
		"image/jpeg":                true,
		"image/png":                 true,
		"image/gif":                 true,
		"image/webp":                true,
		"text/plain; charset=utf-8": true,
	}
)
