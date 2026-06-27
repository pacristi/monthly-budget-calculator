package bootstrap

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
	"presupuesto/internal/ajustes"
	"presupuesto/internal/cartola/compuesto"
	"presupuesto/internal/cartola/ingesta"
	"presupuesto/internal/cartola/obchile"
	sqlitepkg "presupuesto/internal/cartola/sqlite"
	"presupuesto/internal/config"
	"presupuesto/internal/presupuesto"
)

type Config struct {
	Proveedor       string
	ConfigsPath     string
	CategoriasPath  string
	DBPath          string
	ProvisorioPath  string
	DivisionesPath  string
	ExclusionesPath string
	ReglasPath      string
	SueldoPath      string
	ManualesPath    string
	LegacyJSONPath  string
}

type App struct {
	Proveedor       string
	RepoConfigs     *config.RepoJSON
	RepoCategorias  *config.RepoCategorias
	RepoMovimientos ingesta.RepositorioMovimientos
	DB              *sql.DB
	Adaptador       presupuesto.ProveedorFinanciero
	DivisionesPath  string
	ExclusionesPath string
	ReglasPath      string
	SueldoPath      string
	ManualesPath    string
	ProvisorioPath  string
}

func New(cfg Config) (*App, error) {
	if cfg.ProvisorioPath == "" {
		cfg.ProvisorioPath = "data/current.json"
	}

	repoConfigs := config.NewRepoJSON(cfg.ConfigsPath)
	if err := config.EnsureSeed(repoConfigs, config.SeedPorDefecto(time.Now())); err != nil {
		return nil, fmt.Errorf("inicializando configs: %w", err)
	}

	reglas, err := ajustes.CargarReglas(cfg.ReglasPath, cfg.ExclusionesPath)
	if err != nil {
		return nil, fmt.Errorf("cargando reglas: %w", err)
	}

	app := &App{
		Proveedor:       cfg.Proveedor,
		RepoConfigs:     repoConfigs,
		RepoCategorias:  config.NewRepoCategorias(cfg.CategoriasPath),
		DivisionesPath:  cfg.DivisionesPath,
		ExclusionesPath: cfg.ExclusionesPath,
		ReglasPath:      cfg.ReglasPath,
		SueldoPath:      cfg.SueldoPath,
		ManualesPath:    cfg.ManualesPath,
		ProvisorioPath:  cfg.ProvisorioPath,
	}

	switch cfg.Proveedor {
	case "obchile":
		app.Adaptador = obchile.NewAdapter(cfg.LegacyJSONPath, cfg.DivisionesPath, reglas, cfg.SueldoPath, cfg.ManualesPath, repoConfigs)
	case "sqlite", "compuesto":
		db, err := sql.Open("sqlite", cfg.DBPath)
		if err != nil {
			return nil, fmt.Errorf("abriendo BD: %w", err)
		}
		if err := sqlitepkg.Up(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("migraciones: %w", err)
		}
		app.DB = db
		app.RepoMovimientos = sqlitepkg.NewWriter(db, "obchile")

		liquidado := sqlitepkg.NewAdapter(db, cfg.DivisionesPath, reglas, cfg.SueldoPath, cfg.ManualesPath, repoConfigs)
		if cfg.Proveedor == "sqlite" {
			app.Adaptador = liquidado
		} else {
			var provisorio presupuesto.ProveedorFinanciero
			if fileExists(cfg.ProvisorioPath) {
				provisorio = obchile.NewAdapterProvisorio(cfg.ProvisorioPath, cfg.DivisionesPath, reglas, repoConfigs)
			}
			app.Adaptador = compuesto.NewDesdeFuentes(liquidado, provisorio)
		}
	default:
		return nil, fmt.Errorf("--proveedor inválido: %s (compuesto | sqlite | obchile)", cfg.Proveedor)
	}

	return app, nil
}

func (a *App) Close() error {
	if a.DB == nil {
		return nil
	}
	return a.DB.Close()
}

func (a *App) PersisteRefresh() bool {
	return a.Proveedor == "sqlite" || a.Proveedor == "compuesto"
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
