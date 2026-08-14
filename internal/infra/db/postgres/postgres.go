package postgres

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/henvic/pgq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

const COUNT_AS_TOTAL string = "count(*) as total"

type connector struct {
	db *pgxpool.Pool
}

type queryTracer struct {
	log *slog.Logger
}

var whitespaceRgx = regexp.MustCompile(`\s+`)

func (tracer *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !strings.Contains(data.SQL, "river_") && !strings.Contains(data.SQL, "begin") && !strings.Contains(data.SQL, "commit") {
		cleanQuery := strings.TrimSpace(whitespaceRgx.ReplaceAllString(data.SQL, " "))
		tracer.log.Info("Executing command sql", "query", cleanQuery, "args", data.Args)
		return ctx
	}

	return ctx
}

func (tracer *queryTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
}

func New(ctx context.Context, dbURL string) (*connector, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}

	config.ConnConfig.Tracer = &queryTracer{
		log: xlog.Logger,
	}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
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

	return &connector{
		db: pool,
	}, nil
}

func (d *connector) Get() *pgxpool.Pool {
	return d.db
}

func (d *connector) Close() {
	d.db.Close()
}

func (d *connector) Ping(ctx context.Context) error {
	return d.db.Ping(ctx)
}

func wrapPaging(p common.Pagination, sb *pgq.SelectBuilder) {
	if p.Size > 0 {
		*sb = sb.Limit(uint64(p.Size))
	}

	if p.Page > 0 {
		*sb = sb.Offset(uint64(p.GetOffset()))
	}
}
