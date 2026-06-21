// Package ingest contiene los tipos comunes para ingestar movimientos crudos
// desde múltiples fuentes (cartolas xlsx, scraper obchile) hacia la capa
// de persistencia sqlite.
package ingest

import "time"

type Instrumento string

const (
	InstrumentoDesconocido     Instrumento = ""
	InstrumentoCuentaCorriente Instrumento = "cuenta_corriente"
	InstrumentoTarjetaCredito  Instrumento = "tarjeta_credito"
)

type Moneda string

const (
	MonedaCLP Moneda = "CLP"
	MonedaUSD Moneda = "USD"
)

// El xlsx de TC nacional trae el monto de UNA cuota; el scraper de OBCL entrega el total.
type MontoRepresenta string

const (
	MontoRepresentaTotal MontoRepresenta = "total"
	MontoRepresentaCuota MontoRepresenta = "cuota"
)

// MovimientoBruto es el DTO canónico que producen los parsers (xlsx, JSON
// del scraper) y consume el writer del sqlite. Representa un movimiento
// tal cual viene de la fuente, con todos los campos relevantes para el
// dominio + un mapa `Raw` con la información adicional específica de la
// fuente.
type MovimientoBruto struct {
	Banco           string
	Source          string
	Fecha           time.Time
	Monto           float64
	Descripcion     string
	Instrumento     Instrumento
	Moneda          Moneda
	MontoRepresenta MontoRepresenta
	CuotaActual     int
	CuotasTotales   int
	IsUSD           bool
	Cuotas          string
	Raw             map[string]any
}
