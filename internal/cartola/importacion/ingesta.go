// Package importacion orquesta la persistencia de movimientos. Es
// agnóstico al banco: recibe MovimientoBruto ya canónico (lo produce el
// paquete de cada banco) y delega el almacenamiento al repositorio recibido.
package importacion

import (
	"presupuesto/internal/cartola/canonico"
)

// FuenteMovimientos produce movimientos canónicos desde una fuente concreta.
type FuenteMovimientos interface {
	LeerMovimientos() ([]canonico.MovimientoBruto, error)
}

// RepositorioMovimientos persiste movimientos canónicos.
type RepositorioMovimientos interface {
	GuardarMovimientos([]canonico.MovimientoBruto) (int, error)
}

// DesdeFuente lee una fuente de movimientos y persiste su salida canónica.
func DesdeFuente(fuente FuenteMovimientos, repo RepositorioMovimientos) (int, error) {
	brutos, err := fuente.LeerMovimientos()
	if err != nil {
		return 0, err
	}
	return Persistir(brutos, repo)
}

// Persistir vuelca los movimientos al repositorio recibido.
func Persistir(brutos []canonico.MovimientoBruto, repo RepositorioMovimientos) (int, error) {
	return repo.GuardarMovimientos(brutos)
}
