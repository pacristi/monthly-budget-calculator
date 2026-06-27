package ingesta

import (
	"errors"
	"testing"

	"presupuesto/internal/cartola/ingest"
)

func TestPersistir_DelegaEnRepositorio(t *testing.T) {
	repo := &fakeRepoMovimientos{insertados: 7}
	batch := []ingest.MovimientoBruto{{Banco: "bchile", Descripcion: "Cafe"}}

	insertados, err := Persistir(batch, repo)
	if err != nil {
		t.Fatalf("Persistir: %v", err)
	}
	if insertados != 7 {
		t.Fatalf("insertados: esperaba 7, obtuve %d", insertados)
	}
	if len(repo.recibidos) != 1 || repo.recibidos[0].Descripcion != "Cafe" {
		t.Fatalf("repo recibió %+v", repo.recibidos)
	}
}

func TestPersistir_PropagaErrorDelRepositorio(t *testing.T) {
	esperado := errors.New("repo caido")
	repo := &fakeRepoMovimientos{err: esperado}

	_, err := Persistir([]ingest.MovimientoBruto{{Banco: "bchile"}}, repo)
	if !errors.Is(err, esperado) {
		t.Fatalf("error: esperaba %v, obtuve %v", esperado, err)
	}
}

func TestDesdeFuente_DelegaMovimientosEnRepositorio(t *testing.T) {
	fuente := fakeFuenteMovimientos{movs: []ingest.MovimientoBruto{{Banco: "bchile", Descripcion: "Cafe"}}}
	repo := &fakeRepoMovimientos{insertados: 1}

	insertados, err := DesdeFuente(fuente, repo)
	if err != nil {
		t.Fatalf("DesdeFuente: %v", err)
	}
	if insertados != 1 {
		t.Fatalf("insertados: esperaba 1, obtuve %d", insertados)
	}
	if len(repo.recibidos) != 1 || repo.recibidos[0].Descripcion != "Cafe" {
		t.Fatalf("repo recibió %+v", repo.recibidos)
	}
}

func TestDesdeFuente_PropagaErrorDeFuente(t *testing.T) {
	esperado := errors.New("fuente caida")
	fuente := fakeFuenteMovimientos{err: esperado}
	repo := &fakeRepoMovimientos{}

	_, err := DesdeFuente(fuente, repo)
	if !errors.Is(err, esperado) {
		t.Fatalf("error: esperaba %v, obtuve %v", esperado, err)
	}
	if len(repo.recibidos) != 0 {
		t.Fatalf("no debió persistir movimientos: %+v", repo.recibidos)
	}
}

type fakeRepoMovimientos struct {
	recibidos  []ingest.MovimientoBruto
	insertados int
	err        error
}

func (r *fakeRepoMovimientos) GuardarMovimientos(movs []ingest.MovimientoBruto) (int, error) {
	r.recibidos = append([]ingest.MovimientoBruto(nil), movs...)
	return r.insertados, r.err
}

type fakeFuenteMovimientos struct {
	movs []ingest.MovimientoBruto
	err  error
}

func (f fakeFuenteMovimientos) LeerMovimientos() ([]ingest.MovimientoBruto, error) {
	return f.movs, f.err
}
