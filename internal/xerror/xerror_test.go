package xerror_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/stretchr/testify/assert"
)

func TestDefineStatusCode(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
		want    int
	}{
		{
			name:    "session error",
			err:     &xerror.ErrorSession{Message: "session"},
			wantMsg: "session",
			want:    http.StatusUnauthorized,
		},
		{
			name:    "permission error",
			err:     &xerror.ErrorPermission{Message: "permission"},
			wantMsg: "permission",
			want:    http.StatusForbidden,
		},
		{
			name:    "not found error",
			err:     &xerror.ErrorNotFound{Message: "not found"},
			wantMsg: "not found",
			want:    http.StatusNotFound,
		},
		{
			name:    "bad req error",
			err:     &xerror.ErrorBadRequest{Message: "bad req"},
			wantMsg: "bad req",
			want:    http.StatusBadRequest,
		},
		{
			name:    "token error",
			err:     &xerror.ErrorToken{Message: "token"},
			wantMsg: "token",
			want:    http.StatusInternalServerError,
		},
		{
			name:    "validation error",
			err:     &xerror.ErrorValidation{Message: "validation"},
			wantMsg: "validation",
			want:    http.StatusBadRequest,
		},
		{
			name:    "decoding error",
			err:     &xerror.ErrorDecodingRequest{Err: errors.New("decoding_err")},
			wantMsg: "error while decoding request: decoding_err",
			want:    http.StatusBadRequest,
		},
		{
			name:    "default error",
			err:     errors.New("some-error"),
			wantMsg: "some-error",
			want:    http.StatusInternalServerError,
		},
		{
			name: "no error",
			err:  nil,
			want: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xerror.DefineStatusCode(tt.err)
			if tt.err != nil {
				assert.Equal(t, tt.wantMsg, tt.err.Error())
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
