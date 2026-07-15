package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/henvic/pgq"
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

const userColumns = "id, username, password, role_id, created_at, updated_at, deleted_at"

func scanUser(row pgx.Row) (*domain.User, error) {
	var model domain.User
	err := row.Scan(
		&model.ID,
		&model.Username,
		&model.Password,
		&model.RoleID,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.DeletedAt,
	)
	if err != nil {
		if err.Error() == pgx.ErrNoRows.Error() {
			return nil, &xerror.ErrorResourceNotFound{Message: "user not found"}
		}
		return nil, err
	}

	return &model, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, newUser domain.User) (*domain.User, error) {
	query := `
        INSERT INTO users (username, password, role_id)
        VALUES ($1, $2, $3)
        RETURNING %s;
    `
	query = fmt.Sprintf(query, userColumns)

	row := r.GetExecutor(ctx).QueryRow(ctx, query, newUser.Username, newUser.Password, newUser.RoleID)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) CreateUserWithRole(ctx context.Context, newUser domain.User, role string) (*domain.User, error) {
	query := `
        INSERT INTO users (username, password, role_id)
        VALUES ($1, $2, (select id from roles where name = $3))
        RETURNING %s;
    `
	query = fmt.Sprintf(query, userColumns)

	row := r.GetExecutor(ctx).QueryRow(ctx, query, newUser.Username, newUser.Password, role)

	return scanUser(row)
}

func (r *UserRepository) FetchUserByParam(ctx context.Context, param domain.FetchUserParam) (*domain.User, error) {
	qb := strings.Builder{}

	var args []any
	fmt.Fprintf(&qb, `
		SELECT %s
        FROM users
        WHERE deleted_at IS NULL `, userColumns)
	argCount := 1

	if param.ID != nil {
		fmt.Fprintf(&qb, "AND id = $%d ", argCount)
		args = append(args, *param.ID)
		argCount++
	}

	if param.Username != nil {
		fmt.Fprintf(&qb, "AND username = $%d ", argCount)
		args = append(args, *param.Username)
		argCount++
	}

	if argCount == 1 && !param.ForUpdate {
		return nil, &xerror.ErrorValidation{Message: "fetch user param required"}
	}

	if param.ForUpdate {
		qb.WriteString("FOR UPDATE")
	}

	row := r.GetExecutor(ctx).QueryRow(ctx, qb.String(), args...)

	return scanUser(row)
}

func (r *UserRepository) Update(ctx context.Context, id int64, param domain.UpdateUserParam) (*domain.User, error) {
	updateQuery := pgq.Update("users").
		Where("deleted_at IS NULL").Where("id = ?", id).Returning(userColumns)
	argCount := 0
	if param.Password != nil {
		updateQuery = updateQuery.Set("password", *param.Password)
		argCount++
	}

	if param.RoleID != nil {
		updateQuery = updateQuery.Set("role_id", *param.RoleID)
		argCount++
	}

	if argCount == 0 {
		return nil, errors.New("nothing to update")
	}

	updateQuery = updateQuery.Set("updated_at", "NOW()")

	query, args, err := updateQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := r.GetExecutor(ctx).QueryRow(ctx, query, args...)
	return scanUser(row)
}

func (r *UserRepository) FindAll(ctx context.Context, param *domain.FindAllUsersParam) ([]domain.User, error) {
	selectQuery := pgq.Select(userColumns).From("users").Where("deleted_at IS NULL")

	if param.UsernameLike != nil {
		selectQuery = selectQuery.Where("username like ?", fmt.Sprint("%", *param.UsernameLike, "%"))
	}

	countQuery, countArgs, err := pgq.Select(COUNT_AS_TOTAL).FromSelect(selectQuery, "u").SQL()
	if err != nil {
		return nil, err
	}

	err = r.GetExecutor(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&param.Pagination.Total)
	if err != nil {
		return nil, err
	}

	param.Pagination.WrapPaging(&selectQuery)
	query, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.GetExecutor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []domain.User{}

	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}

	return users, rows.Err()
}

func (r *UserRepository) Delete(ctx context.Context, id int64) (*domain.User, error) {
	query := "UPDATE users SET deleted_at = NOW() WHERE id = $1 RETURNING %s"
	query = fmt.Sprintf(query, userColumns)

	row := r.GetExecutor(ctx).QueryRow(ctx, query, id)

	return scanUser(row)
}
