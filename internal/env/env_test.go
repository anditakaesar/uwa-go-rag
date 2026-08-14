package env

import (
	"testing"
)

func Test_getAIEmbeddingModel(test *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{"Custom model from env", "text-embedding-3-large", "text-embedding-3-large"},
		{"Empty env falls back to default", "", DefaultAIEmbeddingModel},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			t.Setenv("AI_EMBEDDING_MODEL", tt.envValue)
			if got := getAIEmbeddingModel(); got != tt.expected {
				t.Errorf("getAIEmbeddingModel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func Test_values_IsDevelopment(test *testing.T) {
	test.Parallel()

	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"Lower dev", "dev", true},
		{"Full development", "development", true},
		{"Production", "production", false},
		{"Empty string", "", false},
		{"Random string", "staging", false},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(t *testing.T) {
			v := &Object{Env: tt.envValue}
			if got := v.IsDevelopment(); got != tt.expected {
				t.Errorf("IsDevelopment() for %s = %v, want %v", tt.envValue, got, tt.expected)
			}
		})
	}
}
