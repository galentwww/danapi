package storage

import "testing"

func TestValidatePostgresIdentifierAllowsSafeRoleNames(t *testing.T) {
	for _, name := range []string{"middleware_readonly", "readonly1", "_readonly"} {
		if err := validatePostgresIdentifier(name); err != nil {
			t.Fatalf("validatePostgresIdentifier(%q) returned error: %v", name, err)
		}
	}
}

func TestValidatePostgresIdentifierRejectsUnsafeRoleNames(t *testing.T) {
	for _, name := range []string{"", "1readonly", "read-only", "readonly;drop role middleware", "readonly user"} {
		if err := validatePostgresIdentifier(name); err == nil {
			t.Fatalf("validatePostgresIdentifier(%q) returned nil", name)
		}
	}
}
