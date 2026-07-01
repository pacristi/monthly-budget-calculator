package defjson

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"presupuesto/presupuesto"
)

// RepoCategorias persiste la lista global de categorías en un JSON.
// Thread-safe vía mutex, con escritura atómica (temp + rename).
type RepoCategorias struct {
	ruta string
	mu   sync.Mutex
}

func NewRepoCategorias(ruta string) *RepoCategorias {
	return &RepoCategorias{ruta: ruta}
}

// categoriasPorDefecto es el fallback cuando no hay categorías declaradas:
// la categoría de gasto legacy, para que un presupuesto en formato viejo
// (solo porcentajeParaGastos) siga teniendo a dónde mapear.
func categoriasPorDefecto() []presupuesto.Categoria {
	return []presupuesto.Categoria{
		{ID: presupuesto.CategoriaPorDefecto, Nombre: "Gasto", Tipo: presupuesto.Limite},
	}
}

// Listar devuelve las categorías. Si el archivo no existe o está vacío,
// devuelve la categoría de gasto por defecto (lectura retrocompatible).
func (r *RepoCategorias) Listar() ([]presupuesto.Categoria, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return categoriasPorDefecto(), nil
		}
		return nil, fmt.Errorf("leyendo %s: %w", r.ruta, err)
	}

	var cats []presupuesto.Categoria
	if err := json.Unmarshal(data, &cats); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", r.ruta, err)
	}
	if len(cats) == 0 {
		return categoriasPorDefecto(), nil
	}
	return cats, nil
}

// Guardar reemplaza la lista completa de categorías.
func (r *RepoCategorias) Guardar(cats []presupuesto.Categoria) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cats == nil {
		cats = []presupuesto.Categoria{}
	}
	data, err := json.MarshalIndent(cats, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando categorías: %w", err)
	}

	return escribirArchivoAtomico(r.ruta, data)
}
