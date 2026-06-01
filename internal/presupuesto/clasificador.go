package presupuesto

import "strings"

// Ignorado es el destino especial: el movimiento no cuenta en ninguna categoría.
const Ignorado = "ignorado"

// CategoriaPorDefecto es el id de categoría que recibe un movimiento que no
// matchea ninguna regla ni tiene override: un gasto no identificado es, por las
// dudas, un gasto.
const CategoriaPorDefecto = "gasto"

// Regla asigna un destino (categoría id o Ignorado) a los movimientos cuya
// descripción contiene Patron (case-insensitive).
type Regla struct {
	Patron  string `json:"patron"`
	Destino string `json:"destino"`
}

// Clasificar decide a qué categoría (o Ignorado) pertenece un movimiento.
// Precedencia: override manual > primera regla que matchea > categoría default.
func Clasificar(descripcion, override string, reglas []Regla, categoriaDefault string) string {
	if override != "" {
		return override
	}
	desc := strings.ToLower(descripcion)
	for _, r := range reglas {
		if r.Patron != "" && strings.Contains(desc, strings.ToLower(r.Patron)) {
			return r.Destino
		}
	}
	return categoriaDefault
}
