// Package ingest contiene los tipos comunes para ingestar movimientos crudos
// desde múltiples fuentes (cartolas xlsx, scraper obchile) hacia la capa
// de persistencia sqlite.
package ingest

import "time"

// MovimientoBruto es el DTO canónico que producen los parsers (xlsx, JSON
// del scraper) y consume el writer del sqlite. Representa un movimiento
// tal cual viene de la fuente, con todos los campos relevantes para el
// dominio + un mapa `Raw` con la información adicional específica de la
// fuente.
type MovimientoBruto struct {
	Banco       string
	Source      string
	Fecha       time.Time
	Monto       float64
	Descripcion string
	IsUSD       bool
	Cuotas      string
	Raw         map[string]any
}
