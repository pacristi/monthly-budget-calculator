// Package sqlite implementa la capa de persistencia local de movimientos
// crudos. Incluye un engine de migraciones forward-only.
package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Up aplica todas las migraciones pendientes en orden.
//
// Lee los archivos *.sql del directorio embebido `migrations/`, los ordena
// por nombre, y aplica los que tienen una versión mayor a la registrada
// en `schema_migrations`. Cada migración corre en su propia transacción.
//
// Si la tabla `schema_migrations` no existe, la crea antes de proceder.
func Up(db *sql.DB) error {
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return fmt.Errorf("creando schema_migrations: %w", err)
	}

	applied, err := readAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("leyendo versiones aplicadas: %w", err)
	}

	pending, err := readPendingMigrations(applied)
	if err != nil {
		return fmt.Errorf("listando migraciones pendientes: %w", err)
	}

	for _, m := range pending {
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("aplicando migración %03d: %w", m.version, err)
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func ensureSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT    NOT NULL
	)`)
	return err
}

func readAppliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func readPendingMigrations(applied map[int]bool) ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	var all []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix := strings.SplitN(e.Name(), "_", 2)[0]
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migración con prefijo no numérico: %s", e.Name())
		}
		if applied[version] {
			continue
		}
		body, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return nil, err
		}
		all = append(all, migration{
			version: version,
			name:    e.Name(),
			sql:     string(body),
		})
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].version < all[j].version
	})
	return all, nil
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(m.sql); err != nil {
		return fmt.Errorf("ejecutando SQL: %w", err)
	}
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, datetime('now'))",
		m.version,
	); err != nil {
		return fmt.Errorf("registrando versión: %w", err)
	}
	return tx.Commit()
}
