package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"presupuesto/internal/app/bootstrap"
)

type refreshFake struct {
	persistirRecibido bool
	llamado           bool
	nuevos            int
	err               error
}

func (f *refreshFake) Ejecutar(persistir bool) (int, error) {
	f.llamado = true
	f.persistirRecibido = persistir
	return f.nuevos, f.err
}

func TestRefresh_RechazaNoPost(t *testing.T) {
	refresh := &refreshFake{}
	deps := apiDeps{app: &bootstrap.App{Proveedor: "obchile"}, refresh: refresh}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", rec.Code)
	}
	if refresh.llamado {
		t.Error("no debió correr el refresh en un GET")
	}
}

func TestRefresh_ModoSimpleSoloScrapea(t *testing.T) {
	refresh := &refreshFake{}
	deps := apiDeps{app: &bootstrap.App{Proveedor: "obchile"}, refresh: refresh}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !refresh.llamado {
		t.Error("debió correr el refresh")
	}
	if refresh.persistirRecibido {
		t.Error("modo simple NO debe persistir")
	}
}

func TestRefresh_ModoSqliteScrapeaYVuelca(t *testing.T) {
	refresh := &refreshFake{nuevos: 3}
	deps := apiDeps{app: &bootstrap.App{Proveedor: "sqlite"}, refresh: refresh}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !refresh.llamado || !refresh.persistirRecibido {
		t.Errorf("modo sqlite debe refrescar (%v) y persistir (%v)", refresh.llamado, refresh.persistirRecibido)
	}
}

func TestRefresh_ModoCompuestoScrapeaYVuelca(t *testing.T) {
	refresh := &refreshFake{nuevos: 3}
	deps := apiDeps{app: &bootstrap.App{Proveedor: "compuesto"}, refresh: refresh}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !refresh.llamado || !refresh.persistirRecibido {
		t.Errorf("modo compuesto debe refrescar (%v) y persistir (%v)", refresh.llamado, refresh.persistirRecibido)
	}
}

func TestRefresh_ErrorDeScraperEs500(t *testing.T) {
	deps := apiDeps{app: &bootstrap.App{Proveedor: "obchile"}, refresh: &refreshFake{err: errors.New("boom")}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}
