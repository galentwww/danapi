package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var postgresIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

type ReadOnlyRoleOptions struct {
	User     string
	Password string
}

func EnsureReadOnlyRole(ctx context.Context, db *pgxpool.Pool, options ReadOnlyRoleOptions) error {
	if options.User == "" && options.Password == "" {
		return nil
	}
	if options.User == "" || options.Password == "" {
		return errors.New("database read-only user and password must both be set")
	}
	if err := validatePostgresIdentifier(options.User); err != nil {
		return err
	}

	var databaseName string
	if err := db.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		return fmt.Errorf("read current database: %w", err)
	}
	if err := validatePostgresIdentifier(databaseName); err != nil {
		return fmt.Errorf("current database name is not a safe identifier: %w", err)
	}

	roleIdentifier := pgx.Identifier{options.User}.Sanitize()
	databaseIdentifier := pgx.Identifier{databaseName}.Sanitize()

	var passwordLiteral string
	if err := db.QueryRow(ctx, "select quote_literal($1)", options.Password).Scan(&passwordLiteral); err != nil {
		return fmt.Errorf("quote read-only password: %w", err)
	}

	var exists bool
	if err := db.QueryRow(ctx, "select exists(select 1 from pg_roles where rolname = $1)", options.User).Scan(&exists); err != nil {
		return fmt.Errorf("check read-only role: %w", err)
	}
	if exists {
		if _, err := db.Exec(ctx, fmt.Sprintf("alter role %s with login password %s nosuperuser nocreatedb nocreaterole noreplication", roleIdentifier, passwordLiteral)); err != nil {
			return fmt.Errorf("alter read-only role: %w", err)
		}
	} else {
		if _, err := db.Exec(ctx, fmt.Sprintf("create role %s with login password %s nosuperuser nocreatedb nocreaterole noreplication", roleIdentifier, passwordLiteral)); err != nil {
			return fmt.Errorf("create read-only role: %w", err)
		}
	}

	statements := []string{
		fmt.Sprintf("grant connect on database %s to %s", databaseIdentifier, roleIdentifier),
		fmt.Sprintf("grant usage on schema public to %s", roleIdentifier),
		fmt.Sprintf("revoke create on schema public from %s", roleIdentifier),
		fmt.Sprintf("grant select on all tables in schema public to %s", roleIdentifier),
		fmt.Sprintf("alter default privileges in schema public grant select on tables to %s", roleIdentifier),
	}
	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement); err != nil {
			return fmt.Errorf("grant read-only role privileges: %w", err)
		}
	}
	return nil
}

func validatePostgresIdentifier(value string) error {
	if !postgresIdentifierPattern.MatchString(value) {
		return fmt.Errorf("unsafe postgres identifier %q", value)
	}
	return nil
}
