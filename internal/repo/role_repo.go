package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/jackc/pgx/v5"
)

type RoleRepository struct {
	db IDBExecutor
}

func NewRoleRepository(db IDBExecutor) *RoleRepository {
	return &RoleRepository{
		db: db,
	}
}

func (r *RoleRepository) GetExecutor(ctx context.Context) IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

const roleColumns = "id, name, description, created_at, updated_at, is_system"

func scanRole(row pgx.Row) (*domain.Role, error) {
	var r domain.Role

	err := row.Scan(
		&r.ID,
		&r.Name,
		&r.Description,
		&r.CreatedAt,
		&r.UpdatedAt,
		&r.IsSystem,
	)

	if err != nil {
		return nil, err
	}

	return &r, nil
}

func (r *RoleRepository) FetchRoleByParam(ctx context.Context, param domain.FetchRoleParam) (*domain.Role, error) {
	qb := strings.Builder{}
	var args []any
	fmt.Fprintf(&qb, `
		SELECT %s
		FROM roles WHERE 1=1
	`, roleColumns)

	argCount := 1
	if param.ID != nil {
		fmt.Fprintf(&qb, "AND id = $%d", argCount)
		args = append(args, *param.ID)
		argCount++
	}

	if param.Name != nil {
		fmt.Fprintf(&qb, "AND name = $%d", argCount)
		args = append(args, *param.Name)
		argCount++
	}

	if argCount == 1 {
		return nil, &xerror.ErrorValidation{Message: "fetch role param required"}
	}

	row := r.GetExecutor(ctx).QueryRow(ctx, qb.String(), args...)
	return scanRole(row)
}

func (r *RoleRepository) FetchAll(ctx context.Context, param domain.FetchAllRoleParam) ([]domain.Role, error) {
	qb := strings.Builder{}
	var args []any
	fmt.Fprintf(&qb, `
		SELECT %s
		FROM roles WHERE 1=1
	`, roleColumns)

	argCount := 1
	if param.NameLike != nil {
		fmt.Fprintf(&qb, "AND name like $%d", argCount)
		args = append(args, fmt.Sprintf("%%%s%%", *param.NameLike))
		argCount++
	}

	if param.Pagination.Size > 0 {
		fmt.Fprintf(&qb, " LIMIT $%d", argCount)
		args = append(args, param.Pagination.Size)
		argCount++
	}

	if param.Pagination.Page > 0 {
		fmt.Fprintf(&qb, " OFFSET $%d", argCount)
		args = append(args, param.Pagination.GetOffset())
		argCount++
	}

	if argCount == 1 {
		return nil, &xerror.ErrorValidation{Message: "fetch role param required"}
	}

	rows, err := r.GetExecutor(ctx).Query(ctx, qb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []domain.Role{}

	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *r)
	}

	return roles, rows.Err()
}
