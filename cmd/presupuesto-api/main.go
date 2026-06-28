package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"presupuesto/internal/ajustes"
	"presupuesto/internal/app/bootstrap"
	"presupuesto/internal/cartola/fuentes"
	"presupuesto/internal/cartola/refresh"
	"presupuesto/internal/presentacion"
	"presupuesto/internal/presupuesto"
)

type apiDeps struct {
	app         *bootstrap.App
	refresh     refreshUseCase
	adaptador   presupuesto.ProveedorFinanciero
	movimientos presentacion.Presentador
}

func main() {
	_ = godotenv.Load()

	port := flag.String("port", "8085", "Puerto para el servidor web")
	rutaConfigsFlag := flag.String("configs", "data/configs-mensuales.json", "Ruta del archivo de configs mensuales")
	proveedorFlag := flag.String("proveedor", "compuesto", "Fuente: compuesto (sqlite liquidado + scrape provisorio, default) | sqlite (solo liquidado) | obchile (legacy)")
	dbPathFlag := flag.String("db", "data/movimientos.db", "Ruta al sqlite (modos compuesto/sqlite)")
	provisorioFlag := flag.String("provisorio", "data/current.json", "Ruta al JSON del último scrape (capa provisoria; modo compuesto)")
	divisionesFlag := flag.String("divisiones", "", "Ruta al JSON de divisiones")
	exclusionesFlag := flag.String("exclusiones", "data/exclusiones.json", "Ruta al JSON con substrings de descripciones a ignorar (legacy, se migra a reglas)")
	reglasFlag := flag.String("reglas", "data/reglas.json", "Ruta al JSON de reglas de categorización [{patron,destino}]")
	categoriasFlag := flag.String("categorias", "data/categorias.json", "Ruta al JSON de categorías [{id,nombre,tipo}]")
	sueldoFlag := flag.String("sueldo", "data/sueldo.json", "Ruta al JSON con patrones de descripción que identifican el sueldo")
	manualesFlag := flag.String("manuales", "data/manuales.json", "Ruta al JSON de gastos manuales")
	flag.Parse()

	legacyJSONPath := ""
	divisionesPath := *divisionesFlag
	if *proveedorFlag == "obchile" {
		args := flag.Args()
		if len(args) < 1 {
			log.Fatalf("Uso: presupuesto-api --proveedor obchile [--configs <ruta>] <ruta_archivo_json> [ruta_archivo_divisiones]")
		}
		legacyJSONPath = args[0]
		if len(args) > 1 {
			divisionesPath = args[1]
		}
	}

	app, err := bootstrap.New(bootstrap.Config{
		Proveedor:       *proveedorFlag,
		ConfigsPath:     *rutaConfigsFlag,
		CategoriasPath:  *categoriasFlag,
		DBPath:          *dbPathFlag,
		ProvisorioPath:  *provisorioFlag,
		DivisionesPath:  divisionesPath,
		ExclusionesPath: *exclusionesFlag,
		ReglasPath:      *reglasFlag,
		SueldoPath:      *sueldoFlag,
		ManualesPath:    *manualesFlag,
		LegacyJSONPath:  legacyJSONPath,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	deps := apiDeps{
		app:         app,
		adaptador:   app.Adaptador,
		movimientos: app.Movimientos,
		refresh: refresh.CasoDeUso{
			Scraper:     nodeScraper{Dir: "ingest", Script: "scraper.js", OutputPath: app.ProvisorioPath},
			Fuente:      fuentes.NuevaOpenBankingChile(app.ProvisorioPath),
			Repositorio: app.RepoMovimientos,
		},
	}

	// Servir archivos estáticos. noCache fuerza al navegador a revalidar antes de
	// usar su copia cacheada, así un deploy nuevo se ve al toque incluso en mobile
	// (donde el hard-refresh es engorroso). FileServer ya manda Last-Modified, así
	// que si nada cambió responde 304 (barato); solo baja bytes cuando hay cambios.
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", noCache(fs))

	// API endpoints
	http.HandleFunc("/api/budget", deps.handleBudget)
	http.HandleFunc("/api/projections", deps.handleProjections)
	http.HandleFunc("/api/movements", deps.handleMovements)
	http.HandleFunc("/api/divisions", deps.handleDivisions)
	http.HandleFunc("/api/configs", handlerListar(app.RepoConfigs))
	http.HandleFunc("/api/configs/", handlerSubconfigs(app.RepoConfigs))
	http.HandleFunc("/api/exclusiones", handleListaStrings(app.ExclusionesPath))
	http.HandleFunc("/api/sueldo", handleListaStrings(app.SueldoPath))
	http.HandleFunc("/api/categorias", deps.handleCategorias)
	http.HandleFunc("/api/reglas", deps.handleReglas)
	http.HandleFunc("/api/movimientos/categoria", deps.handleMovimientoCategoria)
	http.HandleFunc("/api/refresh", deps.handleRefresh)

	fmt.Printf("Servidor iniciado en http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

// noCache envuelve un handler para que el navegador revalide siempre antes de
// servir desde su caché, evitando el bug de "JS viejo + HTML nuevo" tras un deploy.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
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

func (deps apiDeps) handleBudget(w http.ResponseWriter, r *http.Request) {
	adaptador := deps.adaptador
	calc := presupuesto.NewCalculadora(adaptador, deps.app.RepoConfigs)

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

	cfg, err := deps.app.RepoConfigs.ParaMes(periodo.Inicio)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categorias, err := deps.app.RepoCategorias.Listar()
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

func (deps apiDeps) handleProjections(w http.ResponseWriter, r *http.Request) {
	adaptador := deps.adaptador

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
	categorias, err := deps.app.RepoCategorias.Listar()
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

func (deps apiDeps) handleMovements(w http.ResponseWriter, r *http.Request) {
	movs, err := deps.movimientos.PresentarMovimientos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movs)
}

func (deps apiDeps) handleDivisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if deps.app.DivisionesPath == "" {
		http.Error(w, "No divisions file configured", http.StatusBadRequest)
		return
	}

	var req ajustes.Override
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ajustes.GuardarMiParte(deps.app.DivisionesPath, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleListaStrings sirve GET y POST sobre un JSON simple de la forma
// `["a", "b", ...]`. Lo usan los endpoints de exclusiones y patrones de sueldo.
func handleListaStrings(ruta string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ruta == "" {
			http.Error(w, "ruta de archivo no configurada", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			lista, err := ajustes.LeerListaStrings(ruta)
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
			if err := ajustes.EscribirListaStrings(ruta, lista); err != nil {
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
