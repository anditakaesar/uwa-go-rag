package repo_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/repo"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

type mockItems struct {
	ctx    context.Context
	mockDB pgxmock.PgxPoolIface
	now    time.Time
}

func setupMocks() (*mockItems, error) {
	mockDB, err := pgxmock.NewPool()
	if err != nil {
		return nil, err
	}

	return &mockItems{
		ctx:    context.Background(),
		mockDB: mockDB,
		now:    time.Now(),
	}, nil
}

func TestUserRepository_GetExecutor(test *testing.T) {
	test.Parallel()

	test.Run("success return from context", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		m.mockDB.ExpectBegin()

		newTx, err := m.mockDB.Begin(m.ctx)
		assert.NoError(t, err)

		ctxWithValue := context.WithValue(m.ctx, common.TxKey, newTx)
		r := repo.NewUserRepository(m.mockDB)

		got := r.GetExecutor(ctxWithValue)
		assert.Equal(t, newTx, got)
	})

	test.Run("success return default", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		r := repo.NewUserRepository(m.mockDB)
		got := r.GetExecutor(m.ctx)
		assert.Equal(t, m.mockDB, got)
	})
}

func TestUserRepository_CreateUser(test *testing.T) {
	test.Parallel()

	const query = `
			INSERT INTO users (username, password)
			VALUES ($1, $2)
			RETURNING id, username, created_at, updated_at, deleted_at;
		`
	newUser := domain.User{
		Username: "user1",
		Password: "password1",
	}

	test.Run("success", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		rows := m.mockDB.NewRows([]string{"id", "username", "created_at", "updated_at", "deleted_at"}).
			AddRow(int64(1), newUser.Username, m.now, nil, nil)
		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(newUser.Username, newUser.Password).
			WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)
		res, err := r.CreateUser(m.ctx, newUser)

		assert.NoError(t, err)
		assert.Equal(t, "user1", res.Username)
		assert.Equal(t, res.CreatedAt, m.now)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(newUser.Username, newUser.Password).
			WillReturnError(errors.New("query_error"))

		r := repo.NewUserRepository(m.mockDB)
		res, err := r.CreateUser(m.ctx, newUser)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

}

func TestUserRepository_CreateAdminUser(test *testing.T) {
	test.Parallel()

	const query = `
			INSERT INTO users (username, password, role)
			VALUES ($1, $2, $3)
			RETURNING id, username, role, created_at, updated_at, deleted_at;
		`
	newUser := domain.User{
		Username: "user1",
		Password: "password1",
	}

	test.Run("success", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		rows := m.mockDB.NewRows([]string{"id", "username", "role", "created_at", "updated_at", "deleted_at"}).
			AddRow(int64(1), newUser.Username, domain.RoleAdmin, m.now, nil, nil)
		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(newUser.Username, newUser.Password, domain.RoleAdmin).
			WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)
		res, err := r.CreateUserAdmin(m.ctx, newUser)

		assert.NoError(t, err)
		assert.Equal(t, "user1", res.Username)
		assert.Equal(t, domain.RoleAdmin, res.Role)
		assert.Equal(t, res.CreatedAt, m.now)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(newUser.Username, newUser.Password, domain.RoleAdmin).
			WillReturnError(errors.New("query_error"))

		r := repo.NewUserRepository(m.mockDB)
		res, err := r.CreateUserAdmin(m.ctx, newUser)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

}

func TestUserRepository_FetchUserByParam(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		userID := int64(1)
		username := "user1"

		const query = `
			SELECT id, username, password, role, created_at, updated_at, deleted_at
			FROM users
			WHERE deleted_at IS NULL AND id = $1 AND username = $2 FOR UPDATE
		`

		rows := m.mockDB.NewRows([]string{
			"id", "username", "password", "role", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			userID, username, "test-pass", "admin", m.now, nil, nil,
		)

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(
				userID,
				username,
			).WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.FetchUserByParam(m.ctx, domain.FetchUserParam{
			ID:        &userID,
			Username:  &username,
			ForUpdate: true,
		})
		assert.NoError(t, gotErr)
		assert.Equal(t, got.ID, userID)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("success only ID no update", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		userID := int64(1)

		const query = `
			SELECT id, username, password, role, created_at, updated_at, deleted_at
			FROM users
			WHERE deleted_at IS NULL AND id = $1
		`

		rows := m.mockDB.NewRows([]string{
			"id", "username", "password", "role", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			userID, "user1", "test-pass", "admin", m.now, nil, nil,
		)

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(
				userID,
			).WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.FetchUserByParam(m.ctx, domain.FetchUserParam{
			ID: &userID,
		})
		assert.NoError(t, gotErr)
		assert.Equal(t, got.ID, userID)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error when fetch to db", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		userID := int64(1)

		const query = `
			SELECT id, username, password, role, created_at, updated_at, deleted_at
			FROM users
			WHERE deleted_at IS NULL AND id = $1
		`

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(
				userID,
			).WillReturnError(errors.New("error_fetchUser"))

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.FetchUserByParam(m.ctx, domain.FetchUserParam{
			ID: &userID,
		})
		assert.Error(t, gotErr)
		assert.Nil(t, got)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error notihg to get", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.FetchUserByParam(m.ctx, domain.FetchUserParam{})

		assert.Error(t, gotErr)
		assert.Nil(t, got)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})
}

func TestUserRepository_Update(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		userID := int64(1)
		hashedPass := "pass"

		const query = `
			UPDATE users SET password = $1, updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL
			RETURNING id, username, password, role, created_at, updated_at, deleted_at
		`

		rows := m.mockDB.NewRows([]string{
			"id", "username", "password", "role", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			userID, "username", "test-pass", "admin", m.now, nil, nil,
		)

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(
				hashedPass,
				userID,
			).WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.Update(m.ctx, userID, domain.UpdateUserParam{
			Password: &hashedPass,
		})

		assert.NoError(t, gotErr)
		assert.Equal(t, userID, got.ID)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error when execute query", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		userID := int64(1)
		hashedPass := "pass"

		const query = `
			UPDATE users SET password = $1, updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL
			RETURNING id, username, password, role, created_at, updated_at, deleted_at
		`

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(
				hashedPass,
				userID,
			).WillReturnError(errors.New("error_Execute"))

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.Update(m.ctx, userID, domain.UpdateUserParam{
			Password: &hashedPass,
		})

		assert.Error(t, gotErr)
		assert.Nil(t, got)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error when nothing to update", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		userID := int64(1)

		r := repo.NewUserRepository(m.mockDB)
		got, gotErr := r.Update(m.ctx, userID, domain.UpdateUserParam{})

		assert.Error(t, gotErr)
		assert.Nil(t, got)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})
}

func TestUserRepository_FindAll(test *testing.T) {
	test.Parallel()

	const query = `
		SELECT id, username, password, role, created_at, updated_at, deleted_at
        FROM users
        WHERE deleted_at IS NULL
		LIMIT $1 OFFSET $2`

	expectUser := domain.User{
		Base: domain.Base{
			ID: 1,
		},
		Username: "user1",
		Password: "user-pass",
		Role:     domain.RoleAdmin,
	}

	param := domain.FindAllUsersParam{
		Pagination: common.Pagination{
			Page: 1,
			Size: 1,
		},
	}

	test.Run("success", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		rows := m.mockDB.NewRows([]string{
			"id", "username", "password", "role", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			int64(1), expectUser.Username, expectUser.Password, expectUser.Role, m.now, nil, nil,
		)

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(param.Pagination.Size, param.Pagination.GetOffset()).
			WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)

		got, err := r.FindAll(m.ctx, param)
		assert.NoError(t, err)

		assert.Equal(t, 1, len(got))
		assert.Equal(t, int64(1), got[0].ID)
		assert.Equal(t, "user1", got[0].Username)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error receive less column", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		rows := m.mockDB.NewRows([]string{
			"id", "username", "password", "role", "created_at", "updated_at",
		}).AddRow(
			int64(1), expectUser.Username, expectUser.Password, expectUser.Role, m.now, nil,
		)

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(param.Pagination.Size, param.Pagination.GetOffset()).
			WillReturnRows(rows)

		r := repo.NewUserRepository(m.mockDB)

		got, err := r.FindAll(m.ctx, param)
		assert.Error(t, err)
		assert.Nil(t, got)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})

	test.Run("error", func(t *testing.T) {
		m, err := setupMocks()
		assert.NoError(t, err)
		defer m.mockDB.Close()

		// rows := m.mockDB.NewRows([]string{
		// 	"id", "username", "password", "role", "created_at", "updated_at", "deleted_at",
		// }).AddRow(
		// 	int64(1), expectUser.Username, expectUser.Password, expectUser.Role, m.now, nil, nil,
		// )

		m.mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(param.Pagination.Size, param.Pagination.GetOffset()).
			WillReturnError(errors.New("error_Fetch"))

		r := repo.NewUserRepository(m.mockDB)

		got, err := r.FindAll(m.ctx, param)
		assert.Error(t, err)
		assert.Nil(t, got)
		assert.NoError(t, m.mockDB.ExpectationsWereMet())
	})
}
