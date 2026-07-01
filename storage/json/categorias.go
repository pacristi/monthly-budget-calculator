package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"presupuesto/internal/presupuesto"
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

	dir := filepath.Dir(r.ruta)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creando dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".categorias-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creando temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("escribiendo temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cerrando temp: %w", err)
	}
	if err := os.Rename(tmpPath, r.ruta); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renombrando temp a destino: %w", err)
	}
	return nil
}
