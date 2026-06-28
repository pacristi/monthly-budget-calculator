package refresh

import "presupuesto/internal/cartola/importacion"

type Scraper interface {
	Ejecutar() error
}

type CasoDeUso struct {
	Scraper     Scraper
	Fuente      importacion.FuenteMovimientos
	Repositorio importacion.RepositorioMovimientos
}

func (c CasoDeUso) Ejecutar(persistir bool) (int, error) {
	if err := c.Scraper.Ejecutar(); err != nil {
		return 0, err
	}
	if !persistir {
		return 0, nil
	}
	return importacion.DesdeFuente(c.Fuente, c.Repositorio)
}
