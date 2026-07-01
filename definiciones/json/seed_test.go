package defjson

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureSeed_RepoVacioSeSeedeaConHistoria(t *testing.T) {
	r := NewRepoJSON(filepath.Join(t.TempDir(), "configs.json"))

	seed := SeedPorDefecto(time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC))
	if err := EnsureSeed(r, seed); err != nil {
		t.Fatalf("ensure seed: %v", err)
	}

	configs, _ := r.Listar()
	if len(configs) != 1 {
		t.Fatalf("esperaba 1 config, obtuve %d", len(configs))
	}
	// Debe arrancar lo suficientemente atrás para cubrir movimientos
	// históricos del scraper / xlsx. El año/mes exacto es decisión del
	// proyecto: validamos que sea anterior al mes actual.
	if configs[0].MesDesde >= "2026-05" {
		t.Errorf("seed debería arrancar antes del mes actual; mesDesde=%s", configs[0].MesDesde)
	}
}

func TestEnsureSeed_RepoConDatosNoToca(t *testing.T) {
	r := NewRepoJSON(filepath.Join(t.TempDir(), "configs.json"))
	existente := ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.40, DiaDeCorteCredito: 20, TasaCambioUSD: 920}
	if err := r.Guardar(existente); err != nil {
		t.Fatalf("guardar existente: %v", err)
	}

	seed := SeedPorDefecto(time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC))
	if err := EnsureSeed(r, seed); err != nil {
		t.Fatalf("ensure seed: %v", err)
	}

	configs, _ := r.Listar()
	if len(configs) != 1 || configs[0].PorcentajeParaGastos != 0.40 {
		t.Errorf("seed no debió tocar datos existentes, got %+v", configs)
	}
}
