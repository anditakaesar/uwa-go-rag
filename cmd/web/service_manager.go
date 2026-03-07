package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"
)

type IService interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Name() string
}

type ServiceManager struct {
	services []IService
}

func (m *ServiceManager) Register(s IService) {
	m.services = append(m.services, s)
}

func (m *ServiceManager) Start(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	for _, svc := range m.services {
		s := svc
		g.Go(func() error {
			xlog.Logger.Info(fmt.Sprintf("Starting Service: %s", s.Name()))
			return svc.Run(gCtx)
		})
	}

	<-gCtx.Done()

	xlog.Logger.Info("Initiating Sequential Shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := len(m.services) - 1; i >= 0; i-- {
		s := m.services[i]
		xlog.Logger.Info(fmt.Sprintf("Shutting Down Service: %s", s.Name()))
		if err := s.Shutdown(shutdownCtx); err != nil {
			xlog.Logger.Error(fmt.Sprintf("Error shutting down %s: %v", s.Name(), err))
		}
	}

	return g.Wait()
}

// web-server
type WebServer struct {
	name   string
	router *chi.Mux
	server *http.Server
	port   string
}

func NewWebServer(name string, router *chi.Mux, port string) *WebServer {
	return &WebServer{
		name:   name,
		router: router,
		port:   port,
	}
}

func (w *WebServer) Name() string {
	return w.name
}

func (w *WebServer) Run(ctx context.Context) error {
	w.server = &http.Server{
		Addr:    w.port,
		Handler: w.router,
	}

	err := w.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (w *WebServer) Shutdown(ctx context.Context) error {
	if w.server == nil {
		return nil
	}

	return w.server.Shutdown(ctx)
}

// db-server
type IDatabaseSvr interface {
	Close()
	Ping(ctx context.Context) error
}

type DBServer struct {
	db     IDatabaseSvr
	name   string
	stopCh chan struct{}
}

func NewDBServer(db IDatabaseSvr) *DBServer {
	return &DBServer{
		db:   db,
		name: "db-client",
	}
}

func (s *DBServer) Run(ctx context.Context) error {
	s.stopCh = make(chan struct{})
	err := s.db.Ping(ctx)
	if err != nil {
		return err
	}
	xlog.Logger.Info("Database Connected")
	<-s.stopCh
	return nil
}

func (s *DBServer) Shutdown(ctx context.Context) error {
	close(s.stopCh)
	s.db.Close()
	xlog.Logger.Info("Database Connection Closed")
	return nil
}

func (s *DBServer) Name() string {
	return s.name
}

// worker
type IWorkerClient interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type WorkerServer struct {
	name   string
	worker IWorkerClient
}

func NewWorkerServer(name string, worker IWorkerClient) *WorkerServer {
	return &WorkerServer{
		name:   name,
		worker: worker,
	}
}

func (w *WorkerServer) Name() string {
	return w.name
}

func (w *WorkerServer) Run(ctx context.Context) error {
	return w.worker.Start(ctx)
}

func (w *WorkerServer) Shutdown(ctx context.Context) error {
	return w.worker.Stop(ctx)
}
