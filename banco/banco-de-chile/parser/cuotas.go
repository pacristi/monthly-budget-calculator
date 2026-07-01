package bchile

import (
	"strconv"
	"strings"
)

// parseCuotas parsea el formato "M/N" (ej: "02/03") a (actual, total). Lo usan
// el canal scraper y la TC nacional. Devuelve (1,1) si está vacío o no parseable.
func parseCuotas(s string) (actual, total int) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 1, 1
	}
	a, errA := strconv.Atoi(strings.TrimSpace(parts[0]))
	tot, errTot := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errA != nil || errTot != nil || tot < 1 {
		return 1, 1
	}
	return a, tot
}
