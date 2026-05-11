package presupuesto

type TipoPago string

const (
	Debito  TipoPago = "DEBITO"
	Credito TipoPago = "CREDITO"
)

type PoliticaCorte struct {
	Tipo       TipoPago
	DiaDeCorte int // Ignorado si Tipo == Debito
}
