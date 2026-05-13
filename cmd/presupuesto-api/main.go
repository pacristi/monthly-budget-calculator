package main

import (
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
	"github.com/pierocristi/monthly-budget-calculator/internal/config"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

var (
	rutaJson       string
	rutaDivisiones string
	repoConfigs    *config.RepoJSON
)

func main() {
	_ = godotenv.Load()

	port := flag.String("port", "8085", "Puerto para el servidor web")
	rutaConfigsFlag := flag.String("configs", "data/configs-mensuales.json", "Ruta del archivo de configs mensuales")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("Uso: presupuesto-api [--configs <ruta>] <ruta_archivo_json> [ruta_archivo_divisiones]")
	}

	rutaJson = args[0]
	rutaDivisiones = ""
	if len(args) > 1 {
		rutaDivisiones = args[1]
	}

	repoConfigs = config.NewRepoJSON(*rutaConfigsFlag)
	if err := config.EnsureSeed(repoConfigs, config.SeedPorDefecto(time.Now())); err != nil {
		log.Fatalf("inicializando configs: %v", err)
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

	fmt.Printf("Servidor iniciado en http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func handleBudget(w http.ResponseWriter, r *http.Request) {
	adaptador := obchile.NewAdapter(rutaJson, rutaDivisiones, "data/manuales.json", repoConfigs)
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

	disponible, err := calc.CalcularDisponible(periodo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sueldo, _ := adaptador.ObtenerSueldoBase(periodo)
	gastos, _ := adaptador.ObtenerGastosValidos(periodo)

	type GastoDetalle struct {
		Fecha       string  `json:"fecha"`
		Descripcion string  `json:"descripcion"`
		Carga       float64 `json:"carga"`
		Cuotas      int     `json:"cuotas"`
	}

	var cargaTotal float64
	var detalles []GastoDetalle
	for _, g := range gastos {
		carga := g.CalcularCargaParaPeriodo(periodo)
		if carga > 0 {
			cargaTotal += carga
			detalles = append(detalles, GastoDetalle{
				Fecha:       g.FechaTransaccion.Format("2006-01-02"),
				Descripcion: g.Descripcion,
				Carga:       carga,
				Cuotas:      g.Cuotas,
			})
		}
	}

	presupuestoTotal := sueldo * cfg.PorcentajeParaGastos

	response := map[string]interface{}{
		"sueldo":            sueldo,
		"presupuesto_total": presupuestoTotal,
		"carga_actual":      cargaTotal,
		"disponible":        disponible,
		"gastos":            detalles,
		"config":            cfg,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleProjections(w http.ResponseWriter, r *http.Request) {
	adaptador := obchile.NewAdapter(rutaJson, rutaDivisiones, "data/manuales.json", repoConfigs)

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

	proyector := presupuesto.NewProyectorPresupuesto()
	proyecciones := proyector.Proyectar(gastos, ahora, mesesHaciaAdelante)

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
	adaptador := obchile.NewAdapter(rutaJson, rutaDivisiones, "data/manuales.json", repoConfigs)
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
	}

	result := make([]MovimientoRes, 0, len(movs))
	for _, m := range movs {
		result = append(result, MovimientoRes{
			Fecha:       m.Fecha.Format("2006-01-02"),
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       m.IsUSD,
			MiParte:     m.MiParte,
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
		if o.Fecha == req.Fecha && o.MontoOriginal == req.MontoOriginal {
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
