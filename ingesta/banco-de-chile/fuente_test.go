package bancodechile_test

import (
	"testing"

	bancodechile "presupuesto/ingesta/banco-de-chile"
)

func TestNuevaCartolaXLSX_ValidaRegistro(t *testing.T) {
	if _, err := bancodechile.NuevaCartolaXLSX("bchile", "tc-nacional", 0, t.TempDir()); err != nil {
		t.Fatalf("NuevaCartolaXLSX: %v", err)
	}
	if _, err := bancodechile.NuevaCartolaXLSX("otro", "tc-nacional", 0, t.TempDir()); err == nil {
		t.Fatal("esperaba error para banco no registrado")
	}
}
