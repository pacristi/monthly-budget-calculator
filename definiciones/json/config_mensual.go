package defjson

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"presupuesto/presupuesto"
)

// RepoJSON persiste configs mensuales en un archivo JSON.
// Es thread-safe vía mutex y hace escritura atómica (temp + rename).
// Satisface presupuesto.ResolvedorConfig.
type RepoJSON struct {
	ruta string
	mu   sync.Mutex
}

func NewRepoJSON(ruta string) *RepoJSON {
	return &RepoJSON{ruta: ruta}
}

// Listar devuelve todas las configs, ordenadas por mesDesde ascendente.
func (r *RepoJSON) Listar() ([]ConfigMensual, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.leerSinLock()
}

func (r *RepoJSON) leerSinLock() ([]ConfigMensual, error) {
	data, err := os.ReadFile(r.ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConfigMensual{}, nil
		}
		return nil, fmt.Errorf("leyendo %s: %w", r.ruta, err)
	}

	var configs []ConfigMensual
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", r.ruta, err)
	}

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].MesDesde < configs[j].MesDesde
	})
	return configs, nil
}

// ParaMes resuelve la config aplicable para el mes dado.
// Implementa presupuesto.ResolvedorConfig.
func (r *RepoJSON) ParaMes(mes time.Time) (presupuesto.ConfigPresupuesto, error) {
	configs, err := r.Listar()
	if err != nil {
		return presupuesto.ConfigPresupuesto{}, err
	}
	return ResolverParaMes(mes, configs)
}

// Guardar crea o reemplaza una config (por mesDesde). Valida antes de escribir.
func (r *RepoJSON) Guardar(c ConfigMensual) error {
	if err := c.Validar(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	configs, err := r.leerSinLock()
	if err != nil {
		return err
	}

	reemplazado := false
	for i := range configs {
		if configs[i].MesDesde == c.MesDesde {
			configs[i] = c
			reemplazado = true
			break
		}
	}
	if !reemplazado {
		configs = append(configs, c)
	}

	return r.escribirSinLock(configs)
}

// Borrar elimina la config para mesDesde. Falla si quedaría vacía.
func (r *RepoJSON) Borrar(mesDesde string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	configs, err := r.leerSinLock()
	if err != nil {
		return err
	}

	if len(configs) <= 1 {
		return fmt.Errorf("no se puede borrar la única config (rompería la invariante del seed)")
	}

	nuevas := make([]ConfigMensual, 0, len(configs))
	encontrado := false
	for _, c := range configs {
		if c.MesDesde == mesDesde {
			encontrado = true
			continue
		}
		nuevas = append(nuevas, c)
	}
	if !encontrado {
		return fmt.Errorf("no existe config con mesDesde=%s", mesDesde)
	}

	return r.escribirSinLock(nuevas)
}

func (r *RepoJSON) escribirSinLock(configs []ConfigMensual) error {
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].MesDesde < configs[j].MesDesde
	})

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando configs: %w", err)
	}

	return escribirArchivoAtomico(r.ruta, data)
}
