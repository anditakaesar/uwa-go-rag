package infra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type database struct {
	db *pgxpool.Pool
}

type queryTracer struct {
	log *slog.Logger
}

func (tracer *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !strings.Contains(data.SQL, "river_") && !strings.Contains(data.SQL, "begin") && !strings.Contains(data.SQL, "commit") {
		tracer.log.Info(fmt.Sprintf("Executing command sql: %s, args: %v+", data.SQL, data.Args))
		return ctx
	}

	return ctx
}

func (tracer *queryTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
}

func NewDatabase(ctx context.Context, dbURL string) (*database, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}

	config.ConnConfig.Tracer = &queryTracer{
		log: xlog.Logger,
	}

	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &database{
		db: pool,
	}, nil
}

func (d *database) Get() *pgxpool.Pool {
	return d.db
}

func (d *database) Close() {
	d.db.Close()
}

func (d *database) Ping(ctx context.Context) error {
	return d.db.Ping(ctx)
}

// Unit of work
type IInfraDB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Close()
	Ping(ctx context.Context) error
}

type unitOfWork struct {
	db IInfraDB
}

func (u *unitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := u.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rollbackErr := tx.Rollback(rollbackCtx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			xlog.Logger.Error(fmt.Sprintf("rollback err: %v", rollbackErr))
		}
	}()

	txCtx := context.WithValue(ctx, common.TxKey, tx)

	err = fn(txCtx)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func NewUnitOfWork(idb IInfraDB) *unitOfWork {
	return &unitOfWork{db: idb}
}
