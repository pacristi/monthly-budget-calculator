package refresh

import "presupuesto/internal/cartola/ingesta"

type Scraper interface {
	Ejecutar() error
}

type CasoDeUso struct {
	Scraper     Scraper
	Fuente      ingesta.FuenteMovimientos
	Repositorio ingesta.RepositorioMovimientos
}

func (c CasoDeUso) Ejecutar(persistir bool) (int, error) {
	if err := c.Scraper.Ejecutar(); err != nil {
		return 0, err
	}
	if !persistir {
		return 0, nil
	}
	return ingesta.DesdeFuente(c.Fuente, c.Repositorio)
}
