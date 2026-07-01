package ingesta_test

import (
	"errors"
	"strings"
	"testing"

	"presupuesto/ingesta"
	"presupuesto/movimientos"
)

func TestIngestarLeeFuenteYGuarda(t *testing.T) {
	movs := []movimientos.MovimientoBruto{{Descripcion: "CAFE"}}
	fuente := &fuenteFake{movs: movs}
	repo := &repoFake{nuevos: 2}

	nuevos, err := ingesta.Ingestar(fuente, repo)

	if err != nil {
		t.Fatalf("Ingestar: %v", err)
	}
	if nuevos != 2 {
		t.Fatalf("nuevos: %d", nuevos)
	}
	if !fuente.llamada || !repo.llamado {
		t.Fatalf("llamadas fuente=%v repo=%v", fuente.llamada, repo.llamado)
	}
	if len(repo.movs) != 1 || repo.movs[0].Descripcion != "CAFE" {
		t.Fatalf("movimientos guardados: %#v", repo.movs)
	}
}

func TestIngestarPropagaErrorDeFuente(t *testing.T) {
	fuente := &fuenteFake{err: errors.New("fuente rota")}
	repo := &repoFake{}

	_, err := ingesta.Ingestar(fuente, repo)

	if err == nil {
		t.Fatal("esperaba error")
	}
	if !strings.Contains(err.Error(), "fuente rota") {
		t.Fatalf("error inesperado: %v", err)
	}
	if repo.llamado {
		t.Fatal("no debió guardar tras error de fuente")
	}
}

type fuenteFake struct {
	llamada bool
	movs    []movimientos.MovimientoBruto
	err     error
}

func (f *fuenteFake) LeerMovimientos() ([]movimientos.MovimientoBruto, error) {
	f.llamada = true
	return f.movs, f.err
}

type repoFake struct {
	llamado bool
	movs    []movimientos.MovimientoBruto
	nuevos  int
	err     error
}

func (r *repoFake) GuardarMovimientos(movs []movimientos.MovimientoBruto) (int, error) {
	r.llamado = true
	r.movs = movs
	return r.nuevos, r.err
}
