package config

import (
	"testing"
	"time"
)

func mes(t *testing.T, s string) time.Time {
	t.Helper()
	m, err := ParseMes(s)
	if err != nil {
		t.Fatalf("parseando mes %q: %v", s, err)
	}
	return m
}

func TestResolverParaMes_DevuelveConfigExacta(t *testing.T) {
	configs := []ConfigMensual{
		{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950},
	}
	got, err := ResolverParaMes(mes(t, "2026-01"), configs)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.HeredadaDe != "2026-01" || got.PorcentajeParaGastos != 0.25 {
		t.Errorf("se esperaba heredar de 2026-01, got %+v", got)
	}
}

func TestResolverParaMes_CarryForward(t *testing.T) {
	configs := []ConfigMensual{
		{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950},
		{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980},
	}

	// Mes anterior al primer break: hereda de 2026-01
	got, err := ResolverParaMes(mes(t, "2026-03"), configs)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.HeredadaDe != "2026-01" || got.PorcentajeParaGastos != 0.25 {
		t.Errorf("marzo debería heredar 2026-01, got %+v", got)
	}

	// Mes del segundo break: usa 2026-05
	got, err = ResolverParaMes(mes(t, "2026-05"), configs)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.HeredadaDe != "2026-05" || got.PorcentajeParaGastos != 0.30 {
		t.Errorf("mayo debería usar 2026-05, got %+v", got)
	}

	// Mes posterior al último break: sigue heredando de 2026-05
	got, err = ResolverParaMes(mes(t, "2026-09"), configs)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.HeredadaDe != "2026-05" || got.PorcentajeParaGastos != 0.30 {
		t.Errorf("septiembre debería heredar 2026-05, got %+v", got)
	}
}

func TestResolverParaMes_SinConfigAnterior(t *testing.T) {
	configs := []ConfigMensual{
		{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980},
	}
	_, err := ResolverParaMes(mes(t, "2026-01"), configs)
	if err == nil {
		t.Fatal("se esperaba error porque no hay config <= enero")
	}
}

func TestResolverParaMes_VacioRetornaError(t *testing.T) {
	_, err := ResolverParaMes(mes(t, "2026-01"), nil)
	if err == nil {
		t.Fatal("se esperaba error con lista vacía")
	}
}

func TestConfigMensual_Validar(t *testing.T) {
	casos := []struct {
		nombre string
		c      ConfigMensual
		ok     bool
	}{
		{"válida", ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950}, true},
		{"porcentaje negativo", ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: -0.1, DiaDeCorteCredito: 25, TasaCambioUSD: 950}, false},
		{"porcentaje > 1", ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 1.5, DiaDeCorteCredito: 25, TasaCambioUSD: 950}, false},
		{"día corte 0", ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 0, TasaCambioUSD: 950}, false},
		{"día corte 32", ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 32, TasaCambioUSD: 950}, false},
		{"tasa 0", ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 0}, false},
		{"mes inválido", ConfigMensual{MesDesde: "2026/01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950}, false},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			err := tc.c.Validar()
			if tc.ok && err != nil {
				t.Errorf("se esperaba válida, error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("se esperaba error, fue válida")
			}
		})
	}
}
