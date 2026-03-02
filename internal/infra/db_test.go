package infra

import (
	"context"
	"errors"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func TestUnitOfWork_Do(t *testing.T) {
	t.Run("successful transaction", func(t *testing.T) {
		// Create mock
		mock, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mock.Close()

		// Set up expectations
		mock.ExpectBegin()
		mock.ExpectCommit()

		uow := &unitOfWork{db: mock}

		err = uow.Do(context.Background(), func(ctx context.Context) error {
			// Verify context has the transaction key
			tx := ctx.Value(common.TxKey)
			assert.NotNil(t, tx)
			return nil
		})

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback on function error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mock.Close()

		mock.ExpectBegin()
		mock.ExpectRollback()

		uow := &unitOfWork{db: mock}
		dummyErr := errors.New("business logic failed")

		err = uow.Do(context.Background(), func(ctx context.Context) error {
			return dummyErr
		})

		assert.ErrorIs(t, err, dummyErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("failed to begin transaction", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mock.Close()

		mock.ExpectBegin().WillReturnError(errors.New("connection lost"))

		uow := &unitOfWork{db: mock}
		err = uow.Do(context.Background(), func(ctx context.Context) error {
			return nil
		})

		assert.ErrorContains(t, err, "failed to begin transaction")
	})
}
