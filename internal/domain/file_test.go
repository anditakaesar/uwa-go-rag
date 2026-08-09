package domain_test

import (
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

func TestFile_GeneratePublicThumbnailKey(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		f    domain.File
		want string
	}{
		{
			name: "should add public path",
			f: domain.File{
				S3Key: "development/my-file-key.png",
			},
			want: "development/public/my-file-key.webp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.f.GeneratePublicThumbnailKey()
			if got != tt.want {
				t.Errorf("GeneratePublicThumbnailKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
