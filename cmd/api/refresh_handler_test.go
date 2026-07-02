package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// refreshFake stubea la superficie refrescador para testear el handler aislado
// de la ejecución real del scraper.
type refreshFake struct {
	llamado           bool
	persistirRecibido bool
	nuevos            int
	err               error
}

func (f *refreshFake) Refrescar(persistir bool) (int, error) {
	f.llamado = true
	f.persistirRecibido = persistir
	return f.nuevos, f.err
}

func TestRefresh_RechazaNoPost(t *testing.T) {
	refresh := &refreshFake{}
	deps := apiDeps{refresh: refresh}

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

func TestRefresh_PostExitosoPersisteYDevuelveShape(t *testing.T) {
	refresh := &refreshFake{nuevos: 3}
	deps := apiDeps{refresh: refresh}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	if !refresh.llamado {
		t.Error("debió correr el refresh")
	}
	if !refresh.persistirRecibido {
		t.Error("con un solo modo (sqlite), refrescar SIEMPRE persiste")
	}

	var body struct {
		Status string `json:"status"`
		Modo   string `json:"modo"`
		Nuevos int    `json:"nuevos"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" || body.Modo != "sqlite" || body.Nuevos != 3 {
		t.Errorf("shape inesperado: %+v", body)
	}
}

func TestRefresh_ErrorDeRefrescarEs500(t *testing.T) {
	deps := apiDeps{refresh: &refreshFake{err: errors.New("boom")}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	deps.handleRefresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}
