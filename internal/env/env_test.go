package env

import "testing"

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
			v := &values{Env: tt.envValue}
			if got := v.IsDevelopment(); got != tt.expected {
				t.Errorf("IsDevelopment() for %s = %v, want %v", tt.envValue, got, tt.expected)
			}
		})
	}
}
