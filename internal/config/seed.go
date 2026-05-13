package config

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

// SeedPorDefecto construye una config seed con los valores históricos hardcodeados,
// usando como mesDesde el mes actual (truncado).
func SeedPorDefecto(ahora time.Time) ConfigMensual {
	return ConfigMensual{
		MesDesde:             FormatMes(ahora),
		PorcentajeParaGastos: 0.25,
		DiaDeCorteCredito:    25,
		TasaCambioUSD:        950,
	}
}
