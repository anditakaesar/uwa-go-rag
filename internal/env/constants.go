package env

const (
	CSRF_TOKEN_FIELD_NAME        = "csrf_token"
	MAX_UPLOAD_SIZE              = 10 * 1024 * 1024 // 10 MB limit
	SESSION_KEY           string = "auth_session"
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
