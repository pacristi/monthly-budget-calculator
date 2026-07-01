package defjson

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"presupuesto/presupuesto"
)

// manualDTO es la representación persistida de un gasto ingresado a mano
// (manuales.json). No tiene ID de movimiento sqlite/obchile porque no viene
// de ninguna fuente bancaria.
type manualDTO struct {
	ID            string  `json:"id"`
	Descripcion   string  `json:"descripcion"`
	MontoTotal    float64 `json:"montoTotal"`
	CuotasTotales int     `json:"cuotasTotales"`
	FechaInicio   string  `json:"fechaInicio"` // Formato esperado: dd-mm-yyyy
	TipoPago      string  `json:"tipoPago"`    // "debito" o "credito"
}

// RepoGastosManuales lee gastos ingresados a mano por el usuario desde un
// JSON (manuales.json), tolerando archivo ausente.
type RepoGastosManuales struct {
	ruta       string
	resolvedor presupuesto.ResolvedorConfig
}

func NewRepoGastosManuales(ruta string, resolvedor presupuesto.ResolvedorConfig) *RepoGastosManuales {
	return &RepoGastosManuales{ruta: ruta, resolvedor: resolvedor}
}

// Listar lee y mapea los gastos manuales a presupuesto.Gasto, resolviendo la
// política de corte de cada uno vía el ResolvedorConfig del mes correspondiente.
// Tolera archivo ausente (devuelve nil, nil) igual que el resto de repos I/O
// de este paquete.
func (r *RepoGastosManuales) Listar() ([]presupuesto.Gasto, error) {
	if r.ruta == "" {
		return nil, nil
	}
	data, err := os.ReadFile(r.ruta)
	if err != nil {
		return nil, nil // tolerar archivo ausente
	}
	var dtos []manualDTO
	if err := json.Unmarshal(data, &dtos); err != nil {
		return nil, nil
	}

	var out []presupuesto.Gasto
	for _, dto := range dtos {
		fechaTransaccion, err := time.Parse("02-01-2006", dto.FechaInicio)
		if err != nil {
			continue
		}
		cfg, err := r.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, err
		}
		tipo := presupuesto.Debito
		diaCorte := 0
		if strings.ToLower(dto.TipoPago) == "credito" {
			tipo = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}
		out = append(out, presupuesto.Gasto{
			ID:               dto.ID,
			Descripcion:      dto.Descripcion,
			MontoImputado:    dto.MontoTotal,
			Cuotas:           dto.CuotasTotales,
			FechaTransaccion: fechaTransaccion,
			PoliticaCorte:    presupuesto.PoliticaCorte{Tipo: tipo, DiaDeCorte: diaCorte},
			CategoriaID:      presupuesto.CategoriaPorDefecto,
		})
	}
	return out, nil
}
