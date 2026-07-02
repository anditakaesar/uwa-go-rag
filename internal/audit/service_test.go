package audit_test

import (
	"context"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/stretchr/testify/assert"
)

type mockRepository struct {
	inserted bool
	lastLog  audit.AuditLog
}

func (m *mockRepository) Insert(ctx context.Context, auditlog audit.AuditLog) error {
	m.inserted = true
	m.lastLog = auditlog
	return nil
}

func TestAuditLog_Validate(test *testing.T) {
	test.Run("valid audit log", func(t *testing.T) {
		log := audit.AuditLog{
			ResourceName: "users",
			ResourceID:   "123",
			ActorName:    "admin",
		}
		err := log.Validate()
		assert.NoError(t, err)
	})

	test.Run("missing all required fields", func(t *testing.T) {
		log := audit.AuditLog{}
		err := log.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource_name")
		assert.Contains(t, err.Error(), "resource_id")
		assert.Contains(t, err.Error(), "actor_name")
	})

	test.Run("missing resource_id only", func(t *testing.T) {
		log := audit.AuditLog{
			ResourceName: "users",
			ActorName:    "admin",
		}
		err := log.Validate()
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "resource_name")
		assert.Contains(t, err.Error(), "resource_id")
		assert.NotContains(t, err.Error(), "actor_name")
	})
}

func TestAuditRecorder_Record(test *testing.T) {
	repo := &mockRepository{}
	recorder := audit.NewAuditLogRecorder(repo)

	test.Run("successful record", func(t *testing.T) {
		log := audit.AuditLog{
			ResourceName: "users",
			ResourceID:   "123",
			ActorName:    "admin",
		}
		err := recorder.Record(context.Background(), log)
		assert.NoError(t, err)
		assert.True(t, repo.inserted)
		assert.Equal(t, "users", repo.lastLog.ResourceName)
	})

	test.Run("validation failure", func(t *testing.T) {
		repo.inserted = false
		log := audit.AuditLog{
			ResourceName: "users",
		}
		err := recorder.Record(context.Background(), log)
		assert.Error(t, err)
		assert.False(t, repo.inserted)
	})
}
