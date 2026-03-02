package env

import "os"

var Values *values

type values struct {
	Env          string
	Port         string
	DBUrl        string
	CookieSecret string
	CSRFSecret   string
	JWTSecret    string
	PassSecret   string
	UploadDir    string
	HostName     string
}

func init() {
	Values = &values{
		Env:          os.Getenv("ENV"),
		Port:         os.Getenv("PORT"),
		DBUrl:        os.Getenv("DB_URL"),
		CookieSecret: os.Getenv("COOKIE_SECRET"),
		CSRFSecret:   os.Getenv("CSRF_SECRET"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		PassSecret:   os.Getenv("PASS_SECRET"),
		UploadDir:    os.Getenv("UPLOAD_DIR"),
		HostName:     os.Getenv("HOSTNAME"),
	}
}

func (v *values) IsDevelopment() bool {
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
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
)
