// Package ingesta orquesta la extracción de movimientos crudos desde una
// fuente externa y su persistencia en un repositorio.
package ingesta

import "presupuesto/movimientos"

// FuenteMovimientos extrae movimientos crudos desde una fuente externa.
type FuenteMovimientos interface {
	LeerMovimientos() ([]movimientos.MovimientoBruto, error)
}

// RepositorioMovimientos persiste movimientos crudos y devuelve cuántos nuevos
// quedaron almacenados.
type RepositorioMovimientos interface {
	GuardarMovimientos([]movimientos.MovimientoBruto) (int, error)
}

// Ingestar lee los movimientos de la fuente y los guarda en el repositorio,
// devolviendo cuántos nuevos quedaron almacenados.
func Ingestar(f FuenteMovimientos, repo RepositorioMovimientos) (int, error) {
	brutos, err := f.LeerMovimientos()
	if err != nil {
		return 0, err
	}
	return repo.GuardarMovimientos(brutos)
}
