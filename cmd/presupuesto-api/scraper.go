package main

import (
	"fmt"
	"os/exec"
)

type nodeScraper struct {
	Dir        string
	Script     string
	Args       []string
	OutputPath string
}

func (s nodeScraper) Ejecutar() error {
	args := append([]string{s.Script}, s.Args...)
	cmd := exec.Command("node", args...)
	cmd.Dir = s.Dir
	cmd.Env = append(cmd.Environ(), "SCRAPER_OUTPUT_PATH="+s.OutputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scraper: %v: %s", err, out)
	}
	return nil
}
