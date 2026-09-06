// Command migrate applies or reverts database migrations (ADR-0003).
//
// Usage (DATABASE_URL required, ARRIVAL_MIGRATIONS_DIR optional):
//
//	go run ./cmd/migrate up          apply all pending migrations
//	go run ./cmd/migrate status      print current schema version
//	go run ./cmd/migrate down-all    revert ALL migrations (guarded, see below)
//
// Safety: down-all is destructive and only meant for throwaway databases and
// rollback rehearsal. It refuses to run unless ARRIVAL_ALLOW_DESTRUCTIVE_DOWN=1
// is explicitly set (execution plan §7.3: 有业务数据的破坏性 down 禁止自动执行).
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: DATABASE_URL is required")
		os.Exit(2)
	}
	dir, err := store.MigrationsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "up":
		if err := store.MigrateUp(dsn, dir); err != nil {
			fail(err)
		}
		fmt.Println("migrations applied:", dir)
	case "down-all":
		if os.Getenv("ARRIVAL_ALLOW_DESTRUCTIVE_DOWN") != "1" {
			fmt.Fprintln(os.Stderr,
				"refused: down-all is destructive; set ARRIVAL_ALLOW_DESTRUCTIVE_DOWN=1 "+
					"and only ever point it at a throwaway database (execution plan §7.3)")
			os.Exit(3)
		}
		if err := store.MigrateDownAll(dsn, dir); err != nil {
			fail(err)
		}
		fmt.Println("migrations reverted:", dir)
	case "status":
		v, dirty, err := store.MigrateVersion(dsn, dir)
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("no migrations applied")
				return
			}
			fail(err)
		}
		fmt.Printf("version %d (dirty=%v)\n", v, dirty)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate <up|down-all|status>")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
