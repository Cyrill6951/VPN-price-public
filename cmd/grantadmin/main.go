// Command grantadmin sets a user's role by email. Used to bootstrap the first
// administrator: register normally, then run this to promote the account.
//
//	go run ./cmd/grantadmin <email> [role]   # role defaults to superadmin
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: grantadmin <email> [role]")
		os.Exit(2)
	}
	email := os.Args[1]
	role := "superadmin"
	if len(os.Args) >= 3 {
		role = os.Args[2]
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	tag, err := pool.Exec(ctx, `UPDATE users SET role = $2 WHERE email = $1 AND deleted_at IS NULL`, email, role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update:", err)
		os.Exit(1)
	}
	if tag.RowsAffected() == 0 {
		fmt.Fprintf(os.Stderr, "no user with email %q\n", email)
		os.Exit(1)
	}
	fmt.Printf("user %s is now %s\n", email, role)
}
