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

	// schema_migrations debe existir y contener las migraciones aplicadas.
	var version int
	err := db.QueryRow("SELECT version FROM schema_migrations WHERE version = 2").Scan(&version)
	if err != nil {
		t.Fatalf("schema_migrations sin version=2: %v", err)
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
	if count != 2 {
		t.Errorf("esperaba 2 filas en schema_migrations después de dos Ups, obtuve %d", count)
	}
}

func TestUp_BackfillCanonicalFacts(t *testing.T) {
	db := openMemDB(t)
	if _, err := db.Exec(`CREATE TABLE movimientos (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    banco        TEXT    NOT NULL,
    source       TEXT    NOT NULL,
    fecha        TEXT    NOT NULL,
    monto        REAL    NOT NULL,
    descripcion  TEXT    NOT NULL,
    is_usd       INTEGER NOT NULL DEFAULT 0,
    cuotas       TEXT    NOT NULL DEFAULT '',
    raw          TEXT    NOT NULL DEFAULT '{}',
    origen       TEXT    NOT NULL,
    fecha_carga  TEXT    NOT NULL
);
CREATE INDEX idx_mov_dedup ON movimientos (banco, source, fecha, monto, descripcion);
CREATE INDEX idx_mov_fecha ON movimientos (fecha);`); err != nil {
		t.Fatalf("creando schema legacy: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (1, datetime('now'))`); err != nil {
		t.Fatalf("registrando version 1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO movimientos
		(banco, source, fecha, monto, descripcion, is_usd, cuotas, origen, fecha_carga)
		VALUES
		('bchile', 'tc_nacional', '2026-05-10', -30000, 'CUOTAS', 0, '00/03', 'test', '2026-05-15T00:00:00Z'),
		('bchile', 'account', '2026-05-11', -10.5, 'USD', 1, '', 'test', '2026-05-15T00:00:00Z')`); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}

	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}

	var instrumento, moneda string
	var cuotasTotales int
	if err := db.QueryRow(`SELECT instrumento, moneda, cuotas_totales FROM movimientos WHERE descripcion = 'CUOTAS'`).
		Scan(&instrumento, &moneda, &cuotasTotales); err != nil {
		t.Fatalf("query cuotas: %v", err)
	}
	if instrumento != "tarjeta_credito" || moneda != "CLP" || cuotasTotales != 3 {
		t.Fatalf("backfill cuotas = (%q, %q, %d), esperaba tarjeta_credito, CLP, 3", instrumento, moneda, cuotasTotales)
	}

	if err := db.QueryRow(`SELECT instrumento, moneda, cuotas_totales FROM movimientos WHERE descripcion = 'USD'`).
		Scan(&instrumento, &moneda, &cuotasTotales); err != nil {
		t.Fatalf("query usd: %v", err)
	}
	if instrumento != "cuenta_corriente" || moneda != "USD" || cuotasTotales != 1 {
		t.Fatalf("backfill usd = (%q, %q, %d), esperaba cuenta_corriente, USD, 1", instrumento, moneda, cuotasTotales)
	}
}
