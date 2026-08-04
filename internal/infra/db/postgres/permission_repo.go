package postgres

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/jackc/pgx/v5"
)

type PermissionRepo struct {
	db DBExecutor
}

func NewPermissionRepo(db DBExecutor) *PermissionRepo {
	return &PermissionRepo{
		db: db,
	}
}

func (r *PermissionRepo) GetExecutor(ctx context.Context) DBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func scanPermission(row pgx.Row) (*domain.Permission, error) {
	var model domain.Permission
	err := row.Scan(
		&model.ID,
		&model.Resource,
		&model.Action,
		&model.Name,
	)
	if err != nil {
		return nil, err
	}

	return &model, nil
}

func (r *PermissionRepo) GetPermissionsByUser(ctx context.Context, userID int64) ([]domain.Permission, error) {
	const query = `
		SELECT "permissions"."id", "permissions"."resource", "permissions"."action", "permissions"."name"
		FROM "role_permissions"
		JOIN "roles" ON "roles"."id" = "role_permissions"."role_id"
		JOIN "permissions" ON "permissions"."id" = "role_permissions"."permission_id"
		JOIN "users" ON "users"."role_id" = "roles"."id"
		WHERE "users"."id" = $1;
	`

	rows, err := r.GetExecutor(ctx).Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := []domain.Permission{}

	for rows.Next() {
		u, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, *u)
	}

	return permissions, rows.Err()
}
