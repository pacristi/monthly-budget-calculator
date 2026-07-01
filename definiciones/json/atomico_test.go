package defjson

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEscribirArchivoAtomico_EscribeContenido(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "datos.json")

	if err := escribirArchivoAtomico(ruta, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("escribiendo: %v", err)
	}

	got, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leyendo: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("esperaba %q, obtuve %q", `{"a":1}`, string(got))
	}
}

func TestEscribirArchivoAtomico_SobreescribeArchivoExistente(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "datos.json")

	if err := escribirArchivoAtomico(ruta, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("primera escritura: %v", err)
	}
	if err := escribirArchivoAtomico(ruta, []byte(`{"a":2}`)); err != nil {
		t.Fatalf("segunda escritura: %v", err)
	}

	got, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leyendo: %v", err)
	}
	if string(got) != `{"a":2}` {
		t.Fatalf("esperaba contenido actualizado %q, obtuve %q", `{"a":2}`, string(got))
	}
}

func TestEscribirArchivoAtomico_NoDejaArchivosTemporales(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "datos.json")

	if err := escribirArchivoAtomico(ruta, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("escribiendo: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("leyendo dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("esperaba solo el archivo final en el directorio, encontré %d entradas", len(entries))
	}
	if entries[0].Name() != "datos.json" {
		t.Fatalf("esperaba datos.json, encontré %q (¿quedó un temp file?)", entries[0].Name())
	}
}

func TestEscribirArchivoAtomico_CreaDirectorioSiNoExiste(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "anidado")
	ruta := filepath.Join(dir, "datos.json")

	if err := escribirArchivoAtomico(ruta, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("escribiendo: %v", err)
	}

	got, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leyendo: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("esperaba %q, obtuve %q", `{"a":1}`, string(got))
	}
}
