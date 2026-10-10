package user

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestAccountRecoveryUniqueConstraintAcrossProviders(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{"postgres duplicate", &pq.Error{Code: "23505"}, true},
		{"wrapped postgres duplicate", fmt.Errorf("update: %w", &pq.Error{Code: "23505"}), true},
		{"libsql duplicate", errors.New("SQLITE_CONSTRAINT_UNIQUE: UNIQUE constraint failed: users.email_hash"), true},
		{"libsql foreign key", errors.New("SQLITE_CONSTRAINT_FOREIGNKEY: FOREIGN KEY constraint failed"), false},
		{"postgres foreign key", &pq.Error{Code: "23503"}, false},
		{"network failure", errors.New("connection reset by peer"), false},
		{"success", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmailHashUniqueConstraint(tt.err); got != tt.want {
				t.Fatalf("unique constraint = %v, want %v", got, tt.want)
			}
		})
	}
}
