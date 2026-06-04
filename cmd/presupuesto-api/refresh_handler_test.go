package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// restaurarSeams guarda y restaura las variables stubbeables + el modo global,
// para no contaminar otros tests del paquete.
func restaurarSeams(t *testing.T) {
	t.Helper()
	scraperOrig := ejecutarScraper
	volcarOrig := volcarASqlite
	proveedorOrig := proveedor
	t.Cleanup(func() {
		ejecutarScraper = scraperOrig
		volcarASqlite = volcarOrig
		proveedor = proveedorOrig
	})
}

func TestRefresh_RechazaNoPost(t *testing.T) {
	restaurarSeams(t)
	llamado := false
	ejecutarScraper = func() error { llamado = true; return nil }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", rec.Code)
	}
	if llamado {
		t.Error("no debió correr el scraper en un GET")
	}
}

func TestRefresh_ModoSimpleSoloScrapea(t *testing.T) {
	restaurarSeams(t)
	proveedor = "obchile"
	scrapeado := false
	volcado := false
	ejecutarScraper = func() error { scrapeado = true; return nil }
	volcarASqlite = func(_, _ string) (int, error) { volcado = true; return 0, nil }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !scrapeado {
		t.Error("debió correr el scraper")
	}
	if volcado {
		t.Error("modo simple NO debe volcar a sqlite")
	}
}

func TestRefresh_ModoSqliteScrapeaYVuelca(t *testing.T) {
	restaurarSeams(t)
	proveedor = "sqlite"
	scrapeado := false
	volcado := false
	ejecutarScraper = func() error { scrapeado = true; return nil }
	volcarASqlite = func(_, _ string) (int, error) { volcado = true; return 3, nil }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !scrapeado || !volcado {
		t.Errorf("modo sqlite debe scrapear (%v) y volcar (%v)", scrapeado, volcado)
	}
}

func TestRefresh_ErrorDeScraperEs500(t *testing.T) {
	restaurarSeams(t)
	proveedor = "obchile"
	ejecutarScraper = func() error { return http.ErrAbortHandler }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	handleRefresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}
