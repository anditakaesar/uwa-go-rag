package env

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Values          *Object
	CorsOptions     *CorsOptions
	S3Config        *S3Config
	WebServerConfig *WebServerConfig
}

var loadOnce = sync.OnceValue(func() Config {
	return Config{
		Values: &Object{
			Env:              os.Getenv("ENV"),
			DBUrl:            os.Getenv("DB_URL"),
			CookieSecret:     os.Getenv("COOKIE_SECRET"),
			CSRFSecret:       os.Getenv("CSRF_SECRET"),
			JWTSecret:        os.Getenv("JWT_SECRET"),
			JWTExpire:        getJWTExpireSession(),
			PassSecret:       os.Getenv("PASS_SECRET"),
			UploadDir:        os.Getenv("UPLOAD_DIR"),
			HostName:         os.Getenv("HOSTNAME"),
			AIBaseURL:        os.Getenv("AI_BASE_URL"),
			AIAPIKey:         os.Getenv("AI_API_KEY"),
			AIEmbeddingModel: getAIEmbeddingModel(),
			LogToFile:        getLogToFile(),
		},

		CorsOptions: &CorsOptions{
			AllowedOrigins:   getCorsOpt("AllowedOrigins"),
			AllowedMethods:   getCorsOpt("AllowedMethods"),
			AllowedHeaders:   getCorsOpt("AllowedHeaders"),
			ExposedHeaders:   getCorsOpt("ExposedHeaders"),
			AllowCredentials: getCorsOptAllowCredentials(),
			MaxAge:           getCorsOptMaxAge(),
		},

		S3Config: &S3Config{
			S3Endpoint:  os.Getenv("S3_ENDPOINT"),
			S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
			S3SecretKey: os.Getenv("S3_SECRET_KEY"),
			S3Region:    os.Getenv("S3_REGION"),
			S3Bucket:    os.Getenv("S3_BUCKET"),
			S3Prefix:    os.Getenv("S3_PREFIX"),
		},

		WebServerConfig: &WebServerConfig{
			UseTLS:       getServerUseTLS(),
			CertFilePath: os.Getenv("SERVER_CERT_FILE"),
			KeyFilePath:  os.Getenv("SERVER_KEY_FILE"),
			Port:         os.Getenv("PORT"),
		},
	}
})

func Get() Config {
	return loadOnce()
}

type Object struct {
	Env              string
	Port             string
	DBUrl            string
	CookieSecret     string
	CSRFSecret       string
	JWTSecret        string
	JWTExpire        int
	PassSecret       string
	UploadDir        string
	HostName         string
	AIBaseURL        string
	AIAPIKey         string
	AIEmbeddingModel string
	LogToFile        bool
}

type CorsOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

type S3Config struct {
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Region    string
	S3Bucket    string
	S3Prefix    string
}

type WebServerConfig struct {
	UseTLS       bool
	CertFilePath string
	KeyFilePath  string
	Port         string
}

func getServerUseTLS() bool {
	str := os.Getenv("SERVER_USE_TLS")
	if strings.HasPrefix(str, "true") {
		return true
	}

	return false
}

func getJWTExpireSession() int {
	str := os.Getenv("JWT_EXPIRE")
	value, err := strconv.Atoi(str)
	if err != nil || value < 10 {
		return 15
	}
	return value
}

func getLogToFile() bool {
	str := os.Getenv("LOG_TO_FILE")
	if str == "true" {
		return true
	}
	return false
}

const DefaultAIEmbeddingModel = "text-embedding-bge-m3"

func getAIEmbeddingModel() string {
	if model := os.Getenv("AI_EMBEDDING_MODEL"); model != "" {
		return model
	}
	return DefaultAIEmbeddingModel
}

func GetLogLevel() slog.Level {
	str := os.Getenv("LOG_LEVEL")
	switch strings.ToLower(str) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelError
	}
}

func (v *Object) IsDevelopment() bool {
	return v.Env == "dev" || v.Env == "development"
}

func getCorsOpt(key string) []string {
	str := os.Getenv(fmt.Sprint("CORS_OPT_", key))
	return strings.Split(str, ";")
}

func getCorsOptAllowCredentials() bool {
	str := os.Getenv(fmt.Sprint("CORS_OPT_", "AllowCredentials"))
	res, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}

	return res
}

func getCorsOptMaxAge() int {
	str := os.Getenv(fmt.Sprint("CORS_OPT_", "MaxAge"))
	res, err := strconv.Atoi(str)
	if err != nil {
		return 300
	}

	return res
}
