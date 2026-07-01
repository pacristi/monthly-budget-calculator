package defjson

import (
	"fmt"
	"os"
	"path/filepath"
)

// escribirArchivoAtomico escribe data en ruta de forma atómica: primero a un
// archivo temporal en el mismo directorio, luego rename sobre el destino
// final. Esto evita dejar un archivo truncado o a medio escribir si el
// proceso muere a mitad de camino. El locking (si el caller lo necesita para
// evitar escrituras concurrentes desde el mismo proceso) sigue siendo
// responsabilidad de cada repo, no de este helper.
func escribirArchivoAtomico(ruta string, data []byte) error {
	dir := filepath.Dir(ruta)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creando dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creando temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("escribiendo temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cerrando temp: %w", err)
	}
	if err := os.Rename(tmpPath, ruta); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renombrando temp a destino: %w", err)
	}
	return nil
}
