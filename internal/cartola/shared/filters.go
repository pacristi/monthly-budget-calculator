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

// EsProvisorio identifica un movimiento que el banco aún puede modificar
// (cargo no facturado, "unbilled"). Es el límite ÚNICO entre las dos capas:
// la ingesta excluye de la persistencia exactamente lo que esta función
// marca true, y la lectura lo sirve en vivo. Definirlo en un solo lugar
// garantiza que no haya ni huecos ni doble conteo entre liquidado y provisorio.
func EsProvisorio(source string) bool {
	return strings.Contains(strings.ToLower(source), "unbilled")
}

// LeerExclusiones lee un JSON con la lista de substrings a ignorar al
// calcular gastos. El formato es `["fintual", "ahorro x", ...]`.
// Tolera que el archivo no exista (retorna lista vacía).
func LeerExclusiones(ruta string) ([]string, error) {
	return leerListaStrings(ruta)
}

// LeerPatronesSueldo lee un JSON con la lista de substrings que
// identifican un depósito de sueldo en la descripción de un movimiento.
// El formato es `["pago de sueldos", "pago:de sueldos", ...]`.
// Tolera archivo inexistente (retorna lista vacía).
func LeerPatronesSueldo(ruta string) ([]string, error) {
	return leerListaStrings(ruta)
}

func leerListaStrings(ruta string) ([]string, error) {
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

// EscribirListaStrings persiste una lista de strings como JSON con
// indentación. Sobreescribe el archivo si ya existe.
func EscribirListaStrings(ruta string, lista []string) error {
	if ruta == "" {
		return nil
	}
	// Asegura una lista vacía como `[]`, no `null`.
	if lista == nil {
		lista = []string{}
	}
	data, err := json.MarshalIndent(lista, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, data, 0644)
}

// CoincidePatronSueldo retorna true si descripcion (case-insensitive)
// contiene alguno de los patrones. Igual semántica que EsGastoIgnorable
// pero conceptualmente distinto (identificación vs filtrado).
func CoincidePatronSueldo(descripcion string, patrones []string) bool {
	desc := strings.ToLower(descripcion)
	for _, p := range patrones {
		if strings.Contains(desc, strings.ToLower(p)) {
			return true
		}
	}
	return false
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
