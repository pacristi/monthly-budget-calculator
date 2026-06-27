package bootstrap_test

import (
	"strings"
	"testing"

	"presupuesto/internal/app/bootstrap"
)

func TestNewObchileRequiereRutaJSON(t *testing.T) {
	_, err := bootstrap.New(bootstrap.Config{
		Proveedor:       "obchile",
		ConfigsPath:     t.TempDir() + "/configs.json",
		ReglasPath:      t.TempDir() + "/reglas.json",
		ExclusionesPath: t.TempDir() + "/exclusiones.json",
	})
	if err == nil {
		t.Fatal("esperaba error")
	}
	if !strings.Contains(err.Error(), "ruta JSON requerida") {
		t.Fatalf("error inesperado: %v", err)
	}
}
