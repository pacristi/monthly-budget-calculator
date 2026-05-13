package config

import (
	"path/filepath"
	"testing"
	"time"
)

func nuevoRepoTmp(t *testing.T) *RepoJSON {
	t.Helper()
	dir := t.TempDir()
	return NewRepoJSON(filepath.Join(dir, "configs.json"))
}

func TestRepoJSON_GuardarYListar(t *testing.T) {
	r := nuevoRepoTmp(t)

	if err := r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950}); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	if err := r.Guardar(ConfigMensual{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980}); err != nil {
		t.Fatalf("guardar: %v", err)
	}

	got, err := r.Listar()
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	if len(got) != 2 || got[0].MesDesde != "2026-01" || got[1].MesDesde != "2026-05" {
		t.Errorf("lista inesperada: %+v", got)
	}
}

func TestRepoJSON_GuardarReemplaza(t *testing.T) {
	r := nuevoRepoTmp(t)
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.40, DiaDeCorteCredito: 25, TasaCambioUSD: 950})

	got, _ := r.Listar()
	if len(got) != 1 || got[0].PorcentajeParaGastos != 0.40 {
		t.Errorf("reemplazo falló: %+v", got)
	}
}

func TestRepoJSON_GuardarValida(t *testing.T) {
	r := nuevoRepoTmp(t)
	err := r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 2.0, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	if err == nil {
		t.Fatal("se esperaba error de validación")
	}
}

func TestRepoJSON_BorrarBloqueaSiEsLaUnica(t *testing.T) {
	r := nuevoRepoTmp(t)
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})

	if err := r.Borrar("2026-01"); err == nil {
		t.Fatal("se esperaba error al borrar la única config")
	}
}

func TestRepoJSON_BorrarOk(t *testing.T) {
	r := nuevoRepoTmp(t)
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980})

	if err := r.Borrar("2026-05"); err != nil {
		t.Fatalf("borrar: %v", err)
	}
	got, _ := r.Listar()
	if len(got) != 1 || got[0].MesDesde != "2026-01" {
		t.Errorf("borrado dejó estado inesperado: %+v", got)
	}
}

func TestRepoJSON_BorrarNoExistente(t *testing.T) {
	r := nuevoRepoTmp(t)
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980})

	if err := r.Borrar("2026-12"); err == nil {
		t.Fatal("se esperaba error al borrar no existente")
	}
}

func TestRepoJSON_ParaMesUsaResolvedor(t *testing.T) {
	r := nuevoRepoTmp(t)
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	_ = r.Guardar(ConfigMensual{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980})

	got, err := r.ParaMes(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("paraMes: %v", err)
	}
	if got.HeredadaDe != "2026-05" || got.PorcentajeParaGastos != 0.30 {
		t.Errorf("resolución inesperada: %+v", got)
	}
}
