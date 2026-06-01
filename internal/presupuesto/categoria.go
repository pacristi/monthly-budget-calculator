package presupuesto

// TipoCategoria define cómo se interpreta el porcentaje de una categoría.
type TipoCategoria string

const (
	// Meta: el % es una meta a alcanzar (ahorro, inversión). Llenar es bueno.
	Meta TipoCategoria = "meta"
	// Limite: el % es un tope a no exceder (gasto). Llenar es malo.
	Limite TipoCategoria = "limite"
)

// Categoria es la identidad estable de un destino de presupuesto. Vive en una
// lista global; los porcentajes que le aplican son por-mes (ver ConfigPresupuesto).
type Categoria struct {
	ID     string        `json:"id"`
	Nombre string        `json:"nombre"`
	Tipo   TipoCategoria `json:"tipo"`
}
