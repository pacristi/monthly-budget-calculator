// Package app es la capa de composición y casos de uso de la aplicación de
// presupuesto. Un solo camino de lectura: sqlite (movimientos/sqlite) +
// enriquecimiento puro (presupuesto), configuración/overrides/reglas en JSON
// (definiciones/json), ingesta vía scraper (ingesta). No hay ramas por
// "proveedor": el modo multi-fuente murió en los pasos 1-5 del refactor.
package app

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	defjson "presupuesto/definiciones/json"
	"presupuesto/movimientos/sqlite"
	"presupuesto/presupuesto"
)

// origenIngesta etiqueta cada inserción del pipeline de ingesta por
// trazabilidad; no participa en la llave de dedup ni afecta el comportamiento.
const origenIngesta = "scraper-obcl"

// Config son las rutas de todos los archivos de estado de la aplicación.
type Config struct {
	ConfigsPath     string
	CategoriasPath  string
	ReglasPath      string
	ExclusionesPath string
	DivisionesPath  string
	SueldoPath      string
	ManualesPath    string
	DBPath          string
	ProvisorioPath  string // solo output del scraper, no se lee en runtime
}

// App compone los repositorios, la conexión sqlite y el estado cacheado que
// los casos de uso necesitan.
type App struct {
	repoConfigs    *defjson.RepoJSON
	repoCategorias *defjson.RepoCategorias
	manuales       *defjson.RepoGastosManuales

	db     *sql.DB
	writer *sqlite.Writer

	divisionesPath  string
	reglasPath      string
	exclusionesPath string
	sueldoPath      string
	provisorioPath  string

	// overrides y reglas se cachean en memoria porque los casos de uso de
	// lectura (ResumenDelMes/Proyecciones/Movimientos) los consultan en cada
	// request. Se recargan tras cada escritura (Guardar*) para que los cambios
	// vía la API se reflejen sin reiniciar el server — mismo patrón que el
	// adapter viejo, que releía tras cada guardado.
	overrides []presupuesto.Override
	reglas    []presupuesto.Regla
}

// New abre la BD, corre migraciones, seedea configs y arma los repos y caches.
func New(cfg Config) (*App, error) {
	repoConfigs := defjson.NewRepoJSON(cfg.ConfigsPath)
	if err := defjson.EnsureSeed(repoConfigs, defjson.SeedPorDefecto(time.Now())); err != nil {
		return nil, fmt.Errorf("inicializando configs: %w", err)
	}

	reglas, err := defjson.CargarReglas(cfg.ReglasPath, cfg.ExclusionesPath)
	if err != nil {
		return nil, fmt.Errorf("cargando reglas: %w", err)
	}

	overrides, err := defjson.LeerOverrides(cfg.DivisionesPath)
	if err != nil {
		return nil, fmt.Errorf("cargando overrides: %w", err)
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("abriendo BD: %w", err)
	}
	if err := sqlite.Up(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migraciones: %w", err)
	}

	return &App{
		repoConfigs:     repoConfigs,
		repoCategorias:  defjson.NewRepoCategorias(cfg.CategoriasPath),
		manuales:        defjson.NewRepoGastosManuales(cfg.ManualesPath, repoConfigs),
		db:              db,
		writer:          sqlite.NewWriter(db, origenIngesta),
		divisionesPath:  cfg.DivisionesPath,
		reglasPath:      cfg.ReglasPath,
		exclusionesPath: cfg.ExclusionesPath,
		sueldoPath:      cfg.SueldoPath,
		provisorioPath:  cfg.ProvisorioPath,
		overrides:       overrides,
		reglas:          reglas,
	}, nil
}

// Close cierra la conexión a la BD.
func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}

// recargarOverrides relee el archivo de divisiones y reemplaza el cache. Se
// llama tras cada escritura de override para que las lecturas siguientes
// reflejen el cambio.
func (a *App) recargarOverrides() error {
	overrides, err := defjson.LeerOverrides(a.divisionesPath)
	if err != nil {
		return err
	}
	a.overrides = overrides
	return nil
}
