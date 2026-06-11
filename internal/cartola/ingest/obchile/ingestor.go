// Package obchile (ingestor) toma el JSON producido por el scraper de
// Open Banking Chile y vuelca los movimientos al sqlite con dedup
// count-based.
package obchile

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
	legacy "github.com/pierocristi/monthly-budget-calculator/internal/cartola/obchile"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/shared"
	sqlitepkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/sqlite"
)

const banco = "bchile"

// Ingestar lee el JSON del scraper en `jsonPath`, inicializa la BD en
// `dbPath` (aplica migraciones si hace falta) y vuelca los movimientos
// aplicando dedup. Retorna la cantidad de filas insertadas en esta
// corrida.
func Ingestar(jsonPath, dbPath string) (int, error) {
	dtos, err := legacy.NewClient(jsonPath).Fetch()
	if err != nil {
		return 0, fmt.Errorf("leyendo JSON %s: %w", jsonPath, err)
	}

	brutos := make([]ingest.MovimientoBruto, 0, len(dtos))
	for _, d := range dtos {
		if shared.EsProvisorio(d.Source) {
			continue // provisorio: capa viva, no se persiste
		}
		b, err := dtoABruto(d)
		if err != nil {
			return 0, fmt.Errorf("mapeando movimiento (fecha=%s desc=%s): %w", d.Fecha, d.Descripcion, err)
		}
		brutos = append(brutos, b)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, fmt.Errorf("abriendo BD %s: %w", dbPath, err)
	}
	defer db.Close()

	if err := sqlitepkg.Up(db); err != nil {
		return 0, fmt.Errorf("aplicando migraciones: %w", err)
	}

	writer := sqlitepkg.NewWriter(db, "obchile")
	return writer.InsertarConDedup(brutos)
}

func dtoABruto(d legacy.MovimientoDTO) (ingest.MovimientoBruto, error) {
	fecha, err := time.Parse("02-01-2006", d.Fecha)
	if err != nil {
		return ingest.MovimientoBruto{}, fmt.Errorf("fecha %q no parseable: %w", d.Fecha, err)
	}

	raw, err := dtoAMap(d)
	if err != nil {
		return ingest.MovimientoBruto{}, err
	}

	return ingest.MovimientoBruto{
		Banco:       banco,
		Source:      d.Source,
		Fecha:       fecha,
		Monto:       d.Monto,
		Descripcion: d.Descripcion,
		IsUSD:       esMontoUSD(d.Monto),
		Cuotas:      d.Installments,
		Raw:         raw,
	}, nil
}

// esMontoUSD aplica la misma heurística que el adapter legacy: un monto
// con parte decimal no nula es USD. (Bchile expresa CLP como enteros.)
func esMontoUSD(m float64) bool {
	return math.Trunc(m) != m
}

func dtoAMap(d legacy.MovimientoDTO) (map[string]any, error) {
	bytes, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}
