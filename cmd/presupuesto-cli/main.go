package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/obchile"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

func main() {
	// Cargar variables de entorno (silencioso si no existe el archivo .env)
	_ = godotenv.Load()

	detalleFlag := flag.Bool("detalle", false, "Mostrar la lista de gastos que impactan este mes")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("Uso: presupuesto-cli [--detalle] <ruta_archivo_json> [ruta_archivo_divisiones]")
	}

	rutaJson := args[0]
	rutaDivisiones := ""
	if len(args) > 1 {
		rutaDivisiones = args[1]
	}

	fmt.Printf("Iniciando Calculadora de Presupuesto Mensual\n")
	fmt.Printf("Leyendo datos de: %s\n", rutaJson)

	// Valores por defecto
	tasaCambioUSD := 950.0
	diaCorteCredito := 25

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

	// Inicializar adaptador
	adaptador := obchile.NewAdapter(rutaJson, rutaDivisiones, "data/manuales.json", tasaCambioUSD, diaCorteCredito)

	// Inicializar calculadora (asumimos que el porcentaje disponible para gasto es 30%)
	porcentajeParaGastos := 0.25
	calc := presupuesto.NewCalculadora(adaptador, porcentajeParaGastos)

	// Definir periodo actual (por ejemplo, el mes en curso)
	ahora := time.Now()
	periodo := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()),
		Fin:    time.Date(ahora.Year(), ahora.Month()+1, 1, 0, 0, 0, 0, ahora.Location()).Add(-time.Nanosecond),
	}

	disponible, err := calc.CalcularDisponible(periodo)
	if err != nil {
		log.Fatalf("Error calculando presupuesto: %v", err)
	}

	sueldo, err := adaptador.ObtenerSueldoBase(periodo)
	if err != nil {
		fmt.Printf("Advertencia: No se pudo obtener el sueldo base: %v\n", err)
	} else {
		fmt.Printf("Sueldo detectado: $%.0f\n", sueldo)
	}

	if *detalleFlag {
		fmt.Println("\n--- Detalle de Gastos Imputados este Mes ---")
		gastos, _ := adaptador.ObtenerGastosValidos(periodo)
		for _, g := range gastos {
			carga := g.CalcularCargaParaPeriodo(periodo)
			if carga > 0 {
				fmt.Printf("[%s] %s | Monto a pagar: $%.0f (Dividido en %d cuotas)\n", g.FechaTransaccion.Format("02/01/2006"), g.Descripcion, carga, g.Cuotas)
			}
		}
		fmt.Println("--------------------------------------------")
	}

	fmt.Printf("Presupuesto para gastos (%.0f%%): $%.0f\n", porcentajeParaGastos*100, sueldo*porcentajeParaGastos)
	fmt.Printf("Carga de gastos actual en el mes: $%.0f\n", (sueldo*porcentajeParaGastos)-disponible)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("DISPONIBLE RESTANTE PARA EL MES: $%.0f\n", disponible)
}
