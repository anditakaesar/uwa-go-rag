package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	db IDBExecutor
}

func NewUserRepository(db IDBExecutor) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetExecutor(ctx context.Context) IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *UserRepository) CreateUser(ctx context.Context, newUser domain.User) (*domain.User, error) {
	const query = `
        INSERT INTO users (username, password)
        VALUES ($1, $2)
        RETURNING id, username, created_at, updated_at, deleted_at;
    `

	var model domain.User

	err := r.GetExecutor(ctx).QueryRow(ctx, query, newUser.Username, newUser.Password).Scan(
		&model.ID,
		&model.Username,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	return &model, nil
}

func (r *UserRepository) CreateUserAdmin(ctx context.Context, newUser domain.User) (*domain.User, error) {
	const query = `
        INSERT INTO users (username, password, role)
        VALUES ($1, $2, $3)
        RETURNING id, username, role, created_at, updated_at, deleted_at;
    `

	var model domain.User

	err := r.GetExecutor(ctx).QueryRow(ctx, query, newUser.Username, newUser.Password, domain.RoleAdmin).Scan(
		&model.ID,
		&model.Username,
		&model.Role,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	return &model, nil
}

func (r *UserRepository) FetchUserByParam(ctx context.Context, param domain.FetchUserParam) (*domain.User, error) {
	query := `
		SELECT id, username, password, role, created_at, updated_at, deleted_at
        FROM users
        WHERE deleted_at IS NULL `
	var args []any
	argCount := 1

	if param.ID != nil {
		query += fmt.Sprintf("AND id = $%d ", argCount)
		args = append(args, *param.ID)
		argCount++
	}

	if param.Username != nil {
		query += fmt.Sprintf("AND username = $%d ", argCount)
		args = append(args, *param.Username)
		argCount++
	}

	if argCount == 1 && !param.ForUpdate {
		return nil, &xerror.ErrorValidation{Message: "fetch user param required"}
	}

	if param.ForUpdate {
		query += "FOR UPDATE"
	}

	var model domain.User

	err := r.GetExecutor(ctx).QueryRow(ctx, query, args...).Scan(
		&model.ID,
		&model.Username,
		&model.Password,
		&model.Role,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	return &model, nil
}

func (r *UserRepository) Update(ctx context.Context, id int64, param domain.UpdateUserParam) (*domain.User, error) {
	query := "UPDATE users SET "
	var args []any
	argCount := 1

	if param.Password != nil {
		query += fmt.Sprintf("password = $%d, ", argCount)
		args = append(args, *param.Password)
		argCount++
	}

	if argCount == 1 {
		return nil, errors.New("nothing to update")
	}

	query += "updated_at = NOW() "
	query += fmt.Sprintf("WHERE id = $%d AND deleted_at IS NULL ", argCount)
	args = append(args, id)
	query += "RETURNING id, username, password, role, created_at, updated_at, deleted_at"

	var model domain.User

	err := r.GetExecutor(ctx).QueryRow(ctx, query, args...).Scan(
		&model.ID,
		&model.Username,
		&model.Password,
		&model.Role,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	return &model, nil
}

func (r *UserRepository) FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.User, error) {
	const query = `
		SELECT id, username, password, role, created_at, updated_at, deleted_at
        FROM users
        WHERE deleted_at IS NULL
		LIMIT $1 OFFSET $2`

	rows, err := r.GetExecutor(ctx).Query(ctx,
		query, param.Pagination.Size, param.Pagination.GetOffset())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []domain.User{}

	for rows.Next() {
		var u domain.User
		err := rows.Scan(
			&u.ID,
			&u.Username,
			&u.Password,
			&u.Role,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}
