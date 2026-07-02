package openbankingchile

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// EjecutarScraper corre el scraper Node de Open Banking Chile, dejando su
// salida en OutputPath.
type EjecutarScraper struct {
	Dir        string
	Script     string
	Args       []string
	OutputPath string
}

func (s EjecutarScraper) Ejecutar() error {
	// OutputPath es relativo al directorio desde donde corre el proceso Go
	// (el operador/flag --provisorio lo piensa así), pero el proceso node
	// arranca con su cwd cambiado a Dir (la carpeta del scraper). Resolvemos
	// a absoluto ANTES de pasarlo, para que no dependa de esa diferencia de
	// working directory.
	outputAbs, err := filepath.Abs(s.OutputPath)
	if err != nil {
		return fmt.Errorf("scraper: resolviendo ruta de output %q: %w", s.OutputPath, err)
	}

	args := append([]string{s.Script}, s.Args...)
	cmd := exec.Command("node", args...)
	cmd.Dir = s.Dir
	cmd.Env = append(cmd.Environ(), "SCRAPER_OUTPUT_PATH="+outputAbs)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scraper: %v: %s", err, out)
	}
	return nil
}
