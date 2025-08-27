package launchr

import (
	"os"
	"syscall"
	"testing"
)

func TestGetenv(t *testing.T) {
	// Set up test environment variables
	_ = syscall.Setenv("TEST_EXISTING_VAR", "existing_value")
	_ = syscall.Setenv("TEST_EMPTY_VAR", "")
	_ = syscall.Setenv("TEST_DEFAULT", "default_from_env")
	defer func() {
		_ = syscall.Unsetenv("TEST_EXISTING_VAR")
		_ = syscall.Unsetenv("TEST_EMPTY_VAR")
		_ = syscall.Unsetenv("TEST_DEFAULT")
	}()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{"dollar sign", "$", "$"},
		{"existing variable", "TEST_EXISTING_VAR", "existing_value"},
		{"non-existing variable", "TEST_NON_EXISTING", ""},

		// ${var-TEST_DEFAULT} - use TEST_DEFAULT if variable doesn't exist
		{"var-TEST_DEFAULT with existing var", "TEST_EXISTING_VAR-fallback", "existing_value"},
		{"var-TEST_DEFAULT with non-existing var", "TEST_NON_EXISTING-fallback", "fallback"},
		{"var-TEST_DEFAULT with empty var", "TEST_EMPTY_VAR-fallback", ""},

		// ${var:-TEST_DEFAULT} - use TEST_DEFAULT if variable doesn't exist or is empty
		{"var:-TEST_DEFAULT with existing var", "TEST_EXISTING_VAR:-fallback", "existing_value"},
		{"var:-TEST_DEFAULT with non-existing var", "TEST_NON_EXISTING:-fallback", "fallback"},
		{"var:-TEST_DEFAULT with empty var", "TEST_EMPTY_VAR:-fallback", "fallback"},

		// ${var+alternative} - use alternative if variable exists
		{"var+alt with existing var", "TEST_EXISTING_VAR+alternative", "alternative"},
		{"var+alt with non-existing var", "TEST_NON_EXISTING+alternative", ""},
		{"var+alt with empty var", "TEST_EMPTY_VAR+alternative", "alternative"},

		// ${var:+alternative} - use alternative if variable exists and is not empty
		{"var:+alt with existing var", "TEST_EXISTING_VAR:+alternative", "alternative"},
		{"var:+alt with non-existing var", "TEST_NON_EXISTING:+alternative", ""},
		{"var:+alt with empty var", "TEST_EMPTY_VAR:+alternative", ""},

		// Variable expansion in TEST_DEFAULT/alternative values
		{"var-$VAR with variable expansion", "TEST_NON_EXISTING-$TEST_DEFAULT", "default_from_env"},
		{"var:-$VAR with variable expansion", "TEST_NON_EXISTING:-$TEST_DEFAULT", "default_from_env"},
		{"var+$VAR with variable expansion", "TEST_EXISTING_VAR+$TEST_DEFAULT", "default_from_env"},
		{"var:+$VAR with variable expansion", "TEST_EXISTING_VAR:+$TEST_DEFAULT", "default_from_env"},

		// Complex case from your example
		{"complex TEST_DEFAULT with variable", "TEST_NON_EXISTING:-MyValue_$TEST_DEFAULT", "MyValue_default_from_env"},

		// Nested expressions - should not recursively expand
		{"nested expression", "TEST_NON_EXISTING:-${TEST_EXISTING_VAR:+fallback}", "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Getenv(tt.input)
			if result != tt.expected {
				t.Errorf("Getenv(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}

	// Test with os.Expand integration
	t.Run("os.Expand integration", func(t *testing.T) {
		testString := "${TEST_NON_EXISTING:-MyValue_$TEST_DEFAULT}"
		result := os.Expand(testString, Getenv)
		expected := "MyValue_default_from_env"
		if result != expected {
			t.Errorf("os.Expand(%q, Getenv) = %q, want %q", testString, result, expected)
		}
	})
}
