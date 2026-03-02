package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_passwordArgon(test *testing.T) {
	secret := "hardly-a-secret"
	test.Run("success generate and compare", func(t *testing.T) {
		ph := NewPasswordHelper(secret)

		rawPassword := "password-to-be-hashed"

		hashed, gotErr := ph.HashPassword(rawPassword)
		assert.NoError(t, gotErr)

		success, err := ph.CheckPassword(rawPassword, hashed)
		assert.Equal(t, true, success)
		assert.NoError(t, err)

		fail, err := ph.CheckPassword("different", hashed)
		assert.Equal(t, false, fail)
		assert.NoError(t, err)
	})

}

func Test_passwordArgon_CheckPassword(test *testing.T) {
	secret := "hardly-a-secret-2"
	test.Run("fail encode hash", func(t *testing.T) {
		ph := NewPasswordHelper(secret)
		got, gotErr := ph.CheckPassword("test-pass", "a$2$")
		assert.Error(t, gotErr)
		assert.Equal(t, false, got)
	})
}
