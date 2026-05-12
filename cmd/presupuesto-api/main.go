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
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

var (
	rutaJson       string
	rutaDivisiones string
)

func main() {
	_ = godotenv.Load()

	port := flag.String("port", "8085", "Puerto para el servidor web")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("Uso: presupuesto-api <ruta_archivo_json> [ruta_archivo_divisiones]")
	}

	rutaJson = args[0]
	rutaDivisiones = ""
	if len(args) > 1 {
		rutaDivisiones = args[1]
	}

	// Servir archivos estáticos
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/api/budget", handleBudget)
	http.HandleFunc("/api/movements", handleMovements)
	http.HandleFunc("/api/divisions", handleDivisions)

	fmt.Printf("Servidor iniciado en http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func getConfig() (float64, int, float64) {
	tasaCambioUSD := 950.0
	diaCorteCredito := 25
	porcentajeParaGastos := 0.25

	if val := os.Getenv("TASA_CAMBIO_USD"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			tasaCambioUSD = parsed
		}
	}
	if val := os.Getenv("DIA_CORTE_CREDITO"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			diaCorteCredito = parsed
		}
	}
	if val := os.Getenv("PORCENTAJE_GASTOS"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			porcentajeParaGastos = parsed
		}
	}
	return tasaCambioUSD, diaCorteCredito, porcentajeParaGastos
}

func handleBudget(w http.ResponseWriter, r *http.Request) {
	tasaCambioUSD, diaCorteCredito, porcentajeParaGastos := getConfig()

	adaptador := obchile.NewAdapter(rutaJson, rutaDivisiones, tasaCambioUSD, diaCorteCredito)
	calc := presupuesto.NewCalculadora(adaptador, porcentajeParaGastos)

	ahora := time.Now()
	periodo := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()),
		Fin:    time.Date(ahora.Year(), ahora.Month()+1, 1, 0, 0, 0, 0, ahora.Location()).Add(-time.Nanosecond),
	}

	disponible, err := calc.CalcularDisponible(periodo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sueldo, _ := adaptador.ObtenerSueldoBase(periodo)
	gastos, _ := adaptador.ObtenerGastosValidos(periodo)
	
	// Filtramos solo los gastos que tienen carga en este periodo para pasarlos al front
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
				Fecha:       g.FechaTransaccion.Format("02-01-2006"),
				Descripcion: g.Descripcion,
				Carga:       carga,
				Cuotas:      g.Cuotas,
			})
		}
	}

	presupuestoTotal := sueldo * porcentajeParaGastos

	response := map[string]interface{}{
		"sueldo":            sueldo,
		"presupuesto_total": presupuestoTotal,
		"carga_actual":      cargaTotal,
		"disponible":        disponible,
		"gastos":            detalles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleMovements(w http.ResponseWriter, r *http.Request) {
	client := obchile.NewClient(rutaJson)
	movs, err := client.Fetch()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Opcional: Obtener overrides para marcar cuáles ya están divididos
	overrides, _ := shared.LeerOverrides(rutaDivisiones)

	type MovimientoRes struct {
		Fecha       string  `json:"fecha"`
		Descripcion string  `json:"descripcion"`
		Monto       float64 `json:"monto"`
		IsUSD       bool    `json:"isUsd"`
		DivididoEn  int     `json:"divididoEn,omitempty"`
	}

	var result []MovimientoRes
	for _, m := range movs {
		if m.Monto >= 0 {
			continue // Solo mostramos gastos (negativos)
		}
		
		divEn := 0
		for _, o := range overrides {
			if o.Fecha == m.Fecha && o.MontoOriginal == m.Monto {
				divEn = o.DivididoEn
				break
			}
		}

		// Chequear si es USD mediante heurística de decimales
		isUsd := false
		if float64(int64(m.Monto)) != m.Monto {
			isUsd = true
		}

		result = append(result, MovimientoRes{
			Fecha:       m.Fecha,
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       isUsd,
			DivididoEn:  divEn,
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
			overrides[i].DivididoEn = req.DivididoEn
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
