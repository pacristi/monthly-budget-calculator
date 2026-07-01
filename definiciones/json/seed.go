package defjson

import "time"

// EnsureSeed garantiza que el repo tenga al menos una config.
// Si está vacío, escribe la semilla provista. Si ya tiene contenido, no hace nada.
type repoEscribible interface {
	Listar() ([]ConfigMensual, error)
	Guardar(ConfigMensual) error
}

func EnsureSeed(repo repoEscribible, seed ConfigMensual) error {
	configs, err := repo.Listar()
	if err != nil {
		return err
	}
	if len(configs) > 0 {
		return nil
	}
	return repo.Guardar(seed)
}

// SeedPorDefecto construye una config seed con los valores históricos
// hardcodeados.
//
// El mesDesde se fija en enero del año 2020. La motivación: los movimientos
// del scraper o del histórico .xls pueden tener fechas de meses anteriores
// al actual (cuotas comprometidas, pagos de TC, etc.). Si el seed solo cubre
// el mes actual, el adapter falla con "no hay declaraciones <= ese mes" al
// resolver la config para esos movimientos viejos.
//
// El usuario puede agregar configs posteriores desde el dashboard; los meses
// que no las tengan heredan de la inmediatamente anterior (carry-forward).
func SeedPorDefecto(ahora time.Time) ConfigMensual {
	return ConfigMensual{
		MesDesde:             "2020-01",
		PorcentajeParaGastos: 0.25,
		DiaDeCorteCredito:    25,
		TasaCambioUSD:        950,
	}
}
