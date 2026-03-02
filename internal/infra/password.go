package infra

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type pwdparams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

type passwordArgon struct {
	params *pwdparams
	secret string
}

func NewPasswordHelper(secret string) *passwordArgon {
	return &passwordArgon{
		params: &pwdparams{
			memory:      64 * 1024,
			iterations:  3,
			parallelism: 2,
			saltLength:  16,
			keyLength:   32,
		},
		secret: secret,
	}

}

func (ph *passwordArgon) HashPassword(password string) (string, error) {
	salt := make([]byte, ph.params.saltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password+ph.secret),
		salt,
		ph.params.iterations,
		ph.params.memory,
		ph.params.parallelism,
		ph.params.keyLength,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, ph.params.memory, ph.params.iterations, ph.params.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

func (ph *passwordArgon) CheckPassword(password string, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var memory, iterations uint32
	var parallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	comparisonHash := argon2.IDKey(
		[]byte(password+ph.secret),
		salt,
		ph.params.iterations,
		ph.params.memory,
		ph.params.parallelism,
		ph.params.keyLength,
	)

	if subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1 {
		return true, nil
	}

	return false, nil
}
