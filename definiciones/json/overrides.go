package defjson

import (
	"encoding/json"
	"os"
	"sync"

	"presupuesto/presupuesto"
)

// RepoOverrides persiste los ajustes manuales del usuario (mi parte,
// categoría, alias, moneda) sobre movimientos. Thread-safe vía mutex, con
// escritura atómica (temp + rename, vía escribirArchivoAtomico).
type RepoOverrides struct {
	ruta string
	mu   sync.Mutex
}

func NewRepoOverrides(ruta string) *RepoOverrides {
	return &RepoOverrides{ruta: ruta}
}

// Leer devuelve los overrides declarados, o una lista vacía si el archivo
// no existe o está vacío.
func (r *RepoOverrides) Leer() ([]presupuesto.Override, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.leerSinLock()
}

func (r *RepoOverrides) leerSinLock() ([]presupuesto.Override, error) {
	if r.ruta == "" {
		return []presupuesto.Override{}, nil
	}
	data, err := os.ReadFile(r.ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return []presupuesto.Override{}, nil
		}
		return []presupuesto.Override{}, nil
	}
	var overrides []presupuesto.Override
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

// GuardarMiParte inserta o actualiza el ajuste de split de un movimiento,
// preservando sus otros ajustes si ya existían.
func (r *RepoOverrides) GuardarMiParte(o presupuesto.Override) error {
	return r.guardar(o, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.MiParte = nuevo.MiParte
	})
}

// GuardarCategoria inserta o actualiza la categoría manual de un movimiento.
// Categoria="" limpia la categoría manual.
func (r *RepoOverrides) GuardarCategoria(o presupuesto.Override) error {
	return r.guardar(o, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.Categoria = nuevo.Categoria
	})
}

// GuardarAlias inserta o actualiza la descripción visible del movimiento.
// Alias="" limpia el alias.
func (r *RepoOverrides) GuardarAlias(o presupuesto.Override) error {
	return r.guardar(o, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.Alias = nuevo.Alias
	})
}

// GuardarMoneda inserta o actualiza la corrección manual de moneda
// (USD/CLP) de un movimiento.
func (r *RepoOverrides) GuardarMoneda(o presupuesto.Override) error {
	return r.guardar(o, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.EsUSD = nuevo.EsUSD
	})
}

func (r *RepoOverrides) guardar(override presupuesto.Override, aplicar func(*presupuesto.Override, presupuesto.Override)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	overrides, err := r.leerSinLock()
	if err != nil {
		overrides = []presupuesto.Override{}
	}

	found := false
	for i := range overrides {
		if presupuesto.MismoOverrideObjetivo(overrides[i], override) {
			aplicar(&overrides[i], override)
			if override.MovimientoID != "" {
				overrides[i].MovimientoID = override.MovimientoID
			}
			found = true
			break
		}
	}
	if !found {
		aplicar(&override, override)
		if override.MiParte == nil && override.Categoria == "" && override.Alias == "" && override.EsUSD == nil {
			return escribirOverrides(r.ruta, overrides)
		}
		overrides = append(overrides, override)
	}

	return escribirOverrides(r.ruta, overrides)
}

func escribirOverrides(ruta string, overrides []presupuesto.Override) error {
	data, err := json.MarshalIndent(overrides, "", "    ")
	if err != nil {
		return err
	}
	return escribirArchivoAtomico(ruta, data)
}
