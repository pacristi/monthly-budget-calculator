package sqlite

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("no se pudo abrir BD en memoria: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUp_AplicaMigraciones_DesdeCero(t *testing.T) {
	db := openMemDB(t)

	if err := Up(db); err != nil {
		t.Fatalf("Up falló: %v", err)
	}

	// schema_migrations debe existir y contener version=1
	var version int
	err := db.QueryRow("SELECT version FROM schema_migrations WHERE version = 1").Scan(&version)
	if err != nil {
		t.Fatalf("schema_migrations sin version=1: %v", err)
	}

	// movimientos debe existir (insert + count debería funcionar)
	_, err = db.Exec(`INSERT INTO movimientos
		(banco, source, fecha, monto, descripcion, origen, fecha_carga)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"bchile", "cta_corriente", "2025-05-15", -1000.0, "Test", "test", "2025-05-15T00:00:00Z")
	if err != nil {
		t.Fatalf("insert en movimientos falló: %v", err)
	}
}

func TestUp_EsIdempotente(t *testing.T) {
	db := openMemDB(t)

	if err := Up(db); err != nil {
		t.Fatalf("primer Up falló: %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("segundo Up falló: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("esperaba 1 fila en schema_migrations después de dos Ups, obtuve %d", count)
	}
}
