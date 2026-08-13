package server

import (
	"github.com/anditakaesar/uwa-go-rag/internal/infra/storage"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Database interface {
	Get() *pgxpool.Pool
	Close()
}

type StorageClient interface {
	Get() *storage.S3Client
}

type ServerDependency struct {
	DB            Database
	StorageClient StorageClient
}

type Executor struct {
	Mux         *chi.Mux
	RiverClient *river.Client[pgx.Tx]
}

func SetupServer(dep *ServerDependency) *Executor {
	router := chi.NewRouter()
	infraSvc := NewInfra(dep.DB.Get(), dep.StorageClient.Get())

	registerStaticRoutes(router)

	apis := newApis(infraSvc)

	router.Group(func(r chi.Router) {
		registerMainRoutes(r, infraSvc, apis)
	})

	router.Route("/api", func(r chi.Router) {
		registerAPIRoutes(r, infraSvc, apis)
	})

	return &Executor{
		Mux:         router,
		RiverClient: infraSvc.RiverClient,
	}
}