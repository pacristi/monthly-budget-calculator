package shared

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// EsGastoIgnorable devuelve true si la descripción contiene alguna de las
// substrings de `exclusiones` (case-insensitive). Las exclusiones son
// personales: típicamente ahorros, traspasos patrimoniales y pagos de
// tarjeta que ya están contabilizados en otra parte.
func EsGastoIgnorable(descripcion string, exclusiones []string) bool {
	desc := strings.ToLower(descripcion)
	for _, e := range exclusiones {
		if strings.Contains(desc, strings.ToLower(e)) {
			return true
		}
	}
	return false
}

// LeerExclusiones lee un JSON con la lista de substrings a ignorar al
// calcular gastos. El formato es `["fintual", "ahorro x", ...]`.
// Tolera que el archivo no exista (retorna lista vacía).
func LeerExclusiones(ruta string) ([]string, error) {
	if ruta == "" {
		return nil, nil
	}
	data, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ParsearCuotas extrae la cantidad total de cuotas del formato "01/03".
func ParsearCuotas(installments string) int {
	if installments == "" {
		return 1
	}
	parts := strings.Split(installments, "/")
	if len(parts) == 2 {
		cuotas, err := strconv.Atoi(parts[1])
		if err == nil && cuotas > 0 {
			return cuotas
		}
	}
	return 1
}
