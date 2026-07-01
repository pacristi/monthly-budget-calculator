package config

import (
	"path/filepath"
	"testing"

	"presupuesto/internal/presupuesto"
)

func TestRepoCategorias_ArchivoAusenteDevuelveGastoPorDefecto(t *testing.T) {
	repo := NewRepoCategorias(filepath.Join(t.TempDir(), "categorias.json"))
	cats, err := repo.Listar()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(cats) != 1 || cats[0].ID != presupuesto.CategoriaPorDefecto || cats[0].Tipo != presupuesto.Limite {
		t.Errorf("esperaba la categoría gasto por defecto, got %+v", cats)
	}
}

func TestRepoCategorias_GuardarYListar(t *testing.T) {
	repo := NewRepoCategorias(filepath.Join(t.TempDir(), "categorias.json"))
	in := []presupuesto.Categoria{
		{ID: "gasto", Nombre: "Gasto", Tipo: presupuesto.Limite},
		{ID: "ahorro", Nombre: "Ahorro", Tipo: presupuesto.Meta},
	}
	if err := repo.Guardar(in); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	got, err := repo.Listar()
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	if len(got) != 2 || got[1].ID != "ahorro" || got[1].Tipo != presupuesto.Meta {
		t.Errorf("roundtrip falló, got %+v", got)
	}
}
