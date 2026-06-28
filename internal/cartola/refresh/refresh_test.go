package refresh

import (
	"errors"
	"testing"

	"presupuesto/internal/cartola/canonico"
)

type scraperFake struct {
	llamado bool
	err     error
}

func (s *scraperFake) Ejecutar() error {
	s.llamado = true
	return s.err
}

type fuenteFake struct {
	llamada bool
	movs    []canonico.MovimientoBruto
	err     error
}

func (f *fuenteFake) LeerMovimientos() ([]canonico.MovimientoBruto, error) {
	f.llamada = true
	return f.movs, f.err
}

type repoFake struct {
	llamado bool
	movs    []canonico.MovimientoBruto
	nuevos  int
	err     error
}

func (r *repoFake) GuardarMovimientos(movs []canonico.MovimientoBruto) (int, error) {
	r.llamado = true
	r.movs = movs
	return r.nuevos, r.err
}

func TestEjecutarSinPersistirSoloCorreScraper(t *testing.T) {
	scraper := &scraperFake{}
	fuente := &fuenteFake{}
	repo := &repoFake{}
	caso := CasoDeUso{Scraper: scraper, Fuente: fuente, Repositorio: repo}

	nuevos, err := caso.Ejecutar(false)

	if err != nil {
		t.Fatalf("Ejecutar: %v", err)
	}
	if nuevos != 0 {
		t.Fatalf("nuevos: %d", nuevos)
	}
	if !scraper.llamado {
		t.Fatal("debió correr scraper")
	}
	if fuente.llamada || repo.llamado {
		t.Fatal("no debió leer fuente ni persistir")
	}
}

func TestEjecutarPersistiendoLeeFuenteYGuarda(t *testing.T) {
	movs := []canonico.MovimientoBruto{{Descripcion: "CAFE"}}
	scraper := &scraperFake{}
	fuente := &fuenteFake{movs: movs}
	repo := &repoFake{nuevos: 2}
	caso := CasoDeUso{Scraper: scraper, Fuente: fuente, Repositorio: repo}

	nuevos, err := caso.Ejecutar(true)

	if err != nil {
		t.Fatalf("Ejecutar: %v", err)
	}
	if nuevos != 2 {
		t.Fatalf("nuevos: %d", nuevos)
	}
	if !scraper.llamado || !fuente.llamada || !repo.llamado {
		t.Fatalf("llamadas scraper=%v fuente=%v repo=%v", scraper.llamado, fuente.llamada, repo.llamado)
	}
	if len(repo.movs) != 1 || repo.movs[0].Descripcion != "CAFE" {
		t.Fatalf("movimientos guardados: %#v", repo.movs)
	}
}

func TestEjecutarCortaSiFallaScraper(t *testing.T) {
	scraper := &scraperFake{err: errors.New("scraper roto")}
	fuente := &fuenteFake{}
	repo := &repoFake{}
	caso := CasoDeUso{Scraper: scraper, Fuente: fuente, Repositorio: repo}

	_, err := caso.Ejecutar(true)

	if err == nil {
		t.Fatal("esperaba error")
	}
	if fuente.llamada || repo.llamado {
		t.Fatal("no debió leer fuente ni persistir tras error de scraper")
	}
}
