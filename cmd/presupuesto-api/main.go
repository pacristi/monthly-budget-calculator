package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/obchile"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/shared"
	sqlitepkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/sqlite"
	"github.com/pierocristi/monthly-budget-calculator/internal/config"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
	_ "modernc.org/sqlite"
)

var (
	rutaJson        string
	rutaDivisiones  string
	rutaExclusiones string
	rutaReglas      string
	rutaCategorias  string
	rutaSueldo      string
	repoConfigs     *config.RepoJSON
	repoCategorias  *config.RepoCategorias
	proveedor       string
	dbPath          string
	rutaManuales    string
	db              *sql.DB
)

func main() {
	_ = godotenv.Load()

	port := flag.String("port", "8085", "Puerto para el servidor web")
	rutaConfigsFlag := flag.String("configs", "data/configs-mensuales.json", "Ruta del archivo de configs mensuales")
	proveedorFlag := flag.String("proveedor", "obchile", "Fuente: obchile (JSON del scraper, default) | sqlite")
	dbPathFlag := flag.String("db", "data/movimientos.db", "Ruta al sqlite (solo si --proveedor=sqlite)")
	divisionesFlag := flag.String("divisiones", "", "Ruta al JSON de divisiones")
	exclusionesFlag := flag.String("exclusiones", "data/exclusiones.json", "Ruta al JSON con substrings de descripciones a ignorar (legacy, se migra a reglas)")
	reglasFlag := flag.String("reglas", "data/reglas.json", "Ruta al JSON de reglas de categorización [{patron,destino}]")
	categoriasFlag := flag.String("categorias", "data/categorias.json", "Ruta al JSON de categorías [{id,nombre,tipo}]")
	sueldoFlag := flag.String("sueldo", "data/sueldo.json", "Ruta al JSON con patrones de descripción que identifican el sueldo")
	manualesFlag := flag.String("manuales", "data/manuales.json", "Ruta al JSON de gastos manuales")
	flag.Parse()

	proveedor = *proveedorFlag
	dbPath = *dbPathFlag
	rutaExclusiones = *exclusionesFlag
	rutaReglas = *reglasFlag
	rutaCategorias = *categoriasFlag
	rutaSueldo = *sueldoFlag
	rutaManuales = *manualesFlag

	repoConfigs = config.NewRepoJSON(*rutaConfigsFlag)
	repoCategorias = config.NewRepoCategorias(rutaCategorias)
	if err := config.EnsureSeed(repoConfigs, config.SeedPorDefecto(time.Now())); err != nil {
		log.Fatalf("inicializando configs: %v", err)
	}

	switch proveedor {
	case "obchile":
		args := flag.Args()
		if len(args) < 1 {
			log.Fatalf("Uso: presupuesto-api [--configs <ruta>] <ruta_archivo_json> [ruta_archivo_divisiones]")
		}
		rutaJson = args[0]
		rutaDivisiones = ""
		if len(args) > 1 {
			rutaDivisiones = args[1]
		}
	case "sqlite":
		rutaDivisiones = *divisionesFlag
		var err error
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			log.Fatalf("abriendo BD: %v", err)
		}
		if err := sqlitepkg.Up(db); err != nil {
			log.Fatalf("migraciones: %v", err)
		}
	default:
		log.Fatalf("--proveedor inválido: %s (obchile | sqlite)", proveedor)
	}

	// Servir archivos estáticos
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/api/budget", handleBudget)
	http.HandleFunc("/api/projections", handleProjections)
	http.HandleFunc("/api/movements", handleMovements)
	http.HandleFunc("/api/divisions", handleDivisions)
	http.HandleFunc("/api/configs", handlerListar(repoConfigs))
	http.HandleFunc("/api/configs/", handlerSubconfigs(repoConfigs))
	http.HandleFunc("/api/exclusiones", handleListaStrings(&rutaExclusiones))
	http.HandleFunc("/api/sueldo", handleListaStrings(&rutaSueldo))
	http.HandleFunc("/api/categorias", handleCategorias)
	http.HandleFunc("/api/reglas", handleReglas)
	http.HandleFunc("/api/movimientos/categoria", handleMovimientoCategoria)

	fmt.Printf("Servidor iniciado en http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func nuevoAdaptador() presupuesto.ProveedorFinanciero {
	reglas, _ := shared.CargarReglas(rutaReglas, rutaExclusiones)
	if proveedor == "sqlite" {
		return sqlitepkg.NewAdapter(db, rutaDivisiones, reglas, rutaSueldo, rutaManuales, repoConfigs)
	}
	return obchile.NewAdapter(rutaJson, rutaDivisiones, reglas, rutaSueldo, rutaManuales, repoConfigs)
}

// idsLimite devuelve el set de ids de categorías de tipo límite (gasto). El
// detalle de "Gastos en el Mes" y la proyección de pasivos se restringen a
// estas: los aportes de meta (ahorro/inversión) no son gasto ni pasivo.
func idsLimite(cats []presupuesto.Categoria) map[string]bool {
	out := make(map[string]bool)
	for _, c := range cats {
		if c.Tipo == presupuesto.Limite {
			out[c.ID] = true
		}
	}
	return out
}

func handleBudget(w http.ResponseWriter, r *http.Request) {
	adaptador := nuevoAdaptador()
	calc := presupuesto.NewCalculadora(adaptador, repoConfigs)

	ahora := time.Now()

	if mStr := r.URL.Query().Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil && m >= 1 && m <= 12 {
			ahora = time.Date(ahora.Year(), time.Month(m), 1, 0, 0, 0, 0, ahora.Location())
		}
	}
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil {
			ahora = time.Date(y, ahora.Month(), 1, 0, 0, 0, 0, ahora.Location())
		}
	}

	periodo := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()),
		Fin:    time.Date(ahora.Year(), ahora.Month()+1, 1, 0, 0, 0, 0, ahora.Location()).Add(-time.Nanosecond),
	}

	cfg, err := repoConfigs.ParaMes(periodo.Inicio)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categorias, err := repoCategorias.Listar()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resumen, err := calc.CalcularResumen(periodo, categorias)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	gastos, _ := adaptador.ObtenerGastosValidos(periodo)

	type GastoDetalle struct {
		Fecha       string  `json:"fecha"`
		Descripcion string  `json:"descripcion"`
		Carga       float64 `json:"carga"`
		Cuotas      int     `json:"cuotas"`
		CategoriaID string  `json:"categoriaId"`
	}

	esLimite := idsLimite(categorias)
	var detalles []GastoDetalle
	for _, g := range gastos {
		if !esLimite[g.CategoriaID] {
			continue // el detalle de gastos es solo de categorías límite
		}
		carga := g.CalcularCargaParaPeriodo(periodo)
		if carga > 0 {
			detalles = append(detalles, GastoDetalle{
				Fecha:       g.FechaTransaccion.Format("2006-01-02"),
				Descripcion: g.Descripcion,
				Carga:       carga,
				Cuotas:      g.Cuotas,
				CategoriaID: g.CategoriaID,
			})
		}
	}

	type CategoriaRes struct {
		ID          string  `json:"id"`
		Nombre      string  `json:"nombre"`
		Tipo        string  `json:"tipo"`
		Porcentaje  float64 `json:"porcentaje"`
		Presupuesto float64 `json:"presupuesto"`
		Acumulado   float64 `json:"acumulado"`
	}

	cats := make([]CategoriaRes, 0, len(resumen.Categorias))
	for _, c := range resumen.Categorias {
		cats = append(cats, CategoriaRes{
			ID:          c.CategoriaID,
			Nombre:      c.Nombre,
			Tipo:        string(c.Tipo),
			Porcentaje:  cfg.Porcentajes[c.CategoriaID],
			Presupuesto: c.Presupuesto,
			Acumulado:   c.Acumulado,
		})
	}

	response := map[string]interface{}{
		"sueldo":     resumen.Sueldo,
		"categorias": cats,
		"sinAsignar": resumen.SinAsignar,
		"gastos":     detalles,
		"config":     cfg,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleProjections(w http.ResponseWriter, r *http.Request) {
	adaptador := nuevoAdaptador()

	mesesHaciaAdelante := 6
	if mStr := r.URL.Query().Get("months"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil && m > 0 {
			mesesHaciaAdelante = m
		}
	}

	ahora := time.Now()
	periodoActual := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()),
		Fin:    time.Date(ahora.Year(), ahora.Month()+1, 1, 0, 0, 0, 0, ahora.Location()).Add(-time.Nanosecond),
	}

	gastos, err := adaptador.ObtenerGastosValidos(periodoActual)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// La proyección de pasivos es solo de gasto (categorías límite); los
	// aportes de meta no son deuda comprometida a futuro.
	categorias, err := repoCategorias.Listar()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	esLimite := idsLimite(categorias)
	gastosLimite := make([]presupuesto.Gasto, 0, len(gastos))
	for _, g := range gastos {
		if esLimite[g.CategoriaID] {
			gastosLimite = append(gastosLimite, g)
		}
	}

	proyector := presupuesto.NewProyectorPresupuesto()
	proyecciones := proyector.Proyectar(gastosLimite, ahora, mesesHaciaAdelante)

	type ProyeccionRes struct {
		Anio              int     `json:"anio"`
		Mes               string  `json:"mes"`
		MesNum            int     `json:"mesNum"`
		TotalComprometido float64 `json:"totalComprometido"`
	}

	var res []ProyeccionRes
	mesesNombres := []string{"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}
	for _, p := range proyecciones {
		res = append(res, ProyeccionRes{
			Anio:              p.Anio,
			Mes:               mesesNombres[p.Mes-1],
			MesNum:            int(p.Mes),
			TotalComprometido: p.TotalComprometido,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func handleMovements(w http.ResponseWriter, r *http.Request) {
	adaptador := nuevoAdaptador()
	movs, err := adaptador.ObtenerMovimientos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type MovimientoRes struct {
		Fecha       string   `json:"fecha"`
		Descripcion string   `json:"descripcion"`
		Monto       float64  `json:"monto"`
		IsUSD       bool     `json:"isUsd"`
		MiParte     *float64 `json:"miParte,omitempty"`
		CategoriaID string   `json:"categoriaId"`
	}

	result := make([]MovimientoRes, 0, len(movs))
	for _, m := range movs {
		result = append(result, MovimientoRes{
			Fecha:       m.Fecha.Format("2006-01-02"),
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       m.IsUSD,
			MiParte:     m.MiParte,
			CategoriaID: m.CategoriaID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleDivisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if rutaDivisiones == "" {
		http.Error(w, "No divisions file configured", http.StatusBadRequest)
		return
	}

	var req shared.Override
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	overrides, err := shared.LeerOverrides(rutaDivisiones)
	if err != nil {
		overrides = []shared.Override{}
	}

	found := false
	for i, o := range overrides {
		if o.Fecha == req.Fecha && o.MontoOriginal == req.MontoOriginal && o.Descripcion == req.Descripcion {
			overrides[i].MiParte = req.MiParte
			found = true
			break
		}
	}
	if !found {
		overrides = append(overrides, req)
	}

	data, err := json.MarshalIndent(overrides, "", "    ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(rutaDivisiones, data, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleListaStrings sirve GET y POST sobre un JSON simple de la forma
// `["a", "b", ...]`. Lo usan los endpoints de exclusiones y patrones de
// sueldo. La ruta se pasa por puntero porque las variables globales pueden
// estar vacías al momento de armar el handler (defaults configurados después).
func handleListaStrings(ruta *string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ruta == nil || *ruta == "" {
			http.Error(w, "ruta de archivo no configurada", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			lista, err := shared.LeerExclusiones(*ruta)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if lista == nil {
				lista = []string{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lista)
		case http.MethodPost:
			var lista []string
			if err := json.NewDecoder(r.Body).Decode(&lista); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := shared.EscribirListaStrings(*ruta, lista); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
