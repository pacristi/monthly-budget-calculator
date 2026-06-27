package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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

// restaurarSeams guarda y restaura las variables stubbeables + el modo global,
// para no contaminar otros tests del paquete.
func restaurarSeams(t *testing.T) {
	t.Helper()
	refreshOrig := refrescarDashboard
	proveedorOrig := proveedor
	t.Cleanup(func() {
		refrescarDashboard = refreshOrig
		proveedor = proveedorOrig
	})
}

func TestRefresh_RechazaNoPost(t *testing.T) {
	restaurarSeams(t)
	refresh := &refreshFake{}
	refrescarDashboard = refresh

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", rec.Code)
	}
	if refresh.llamado {
		t.Error("no debió correr el refresh en un GET")
	}
}

func TestRefresh_ModoSimpleSoloScrapea(t *testing.T) {
	restaurarSeams(t)
	proveedor = "obchile"
	refresh := &refreshFake{}
	refrescarDashboard = refresh

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

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
	restaurarSeams(t)
	proveedor = "sqlite"
	refresh := &refreshFake{nuevos: 3}
	refrescarDashboard = refresh

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !refresh.llamado || !refresh.persistirRecibido {
		t.Errorf("modo sqlite debe refrescar (%v) y persistir (%v)", refresh.llamado, refresh.persistirRecibido)
	}
}

func TestRefresh_ModoCompuestoScrapeaYVuelca(t *testing.T) {
	restaurarSeams(t)
	proveedor = "compuesto"
	refresh := &refreshFake{nuevos: 3}
	refrescarDashboard = refresh

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !refresh.llamado || !refresh.persistirRecibido {
		t.Errorf("modo compuesto debe refrescar (%v) y persistir (%v)", refresh.llamado, refresh.persistirRecibido)
	}
}

func TestRefresh_ErrorDeScraperEs500(t *testing.T) {
	restaurarSeams(t)
	proveedor = "obchile"
	refrescarDashboard = &refreshFake{err: errors.New("boom")}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}
