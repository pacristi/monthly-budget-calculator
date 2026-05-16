package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/obchile"
	"github.com/pierocristi/monthly-budget-calculator/internal/config"
	sqlitepkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/sqlite"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "sqlite" {
		runSqliteSubcommand(os.Args[2:])
		return
	}

	_ = godotenv.Load()

	detalleFlag := flag.Bool("detalle", false, "Mostrar la lista de gastos que impactan este mes")
	proyectarFlag := flag.Int("proyectar", 0, "Proyectar la carga de gastos para los próximos N meses")
	rutaConfigsFlag := flag.String("configs", "data/configs-mensuales.json", "Ruta del archivo de configs mensuales")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("Uso: presupuesto-cli [--detalle] [--proyectar N] [--configs <ruta>] <ruta_archivo_json> [ruta_archivo_divisiones]")
	}

	rutaJson := args[0]
	rutaDivisiones := ""
	if len(args) > 1 {
		rutaDivisiones = args[1]
	}

	fmt.Printf("Iniciando Calculadora de Presupuesto Mensual\n")
	fmt.Printf("Leyendo datos de: %s\n", rutaJson)

	repoConfigs := config.NewRepoJSON(*rutaConfigsFlag)
	if err := config.EnsureSeed(repoConfigs, config.SeedPorDefecto(time.Now())); err != nil {
		log.Fatalf("inicializando configs: %v", err)
	}

	adaptador := obchile.NewAdapter(rutaJson, rutaDivisiones, "data/manuales.json", repoConfigs)
	calc := presupuesto.NewCalculadora(adaptador, repoConfigs)

	ahora := time.Now()
	periodo := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, ahora.Location()),
		Fin:    time.Date(ahora.Year(), ahora.Month()+1, 1, 0, 0, 0, 0, ahora.Location()).Add(-time.Nanosecond),
	}

	cfg, err := repoConfigs.ParaMes(periodo.Inicio)
	if err != nil {
		log.Fatalf("Error resolviendo config del mes: %v", err)
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

	fmt.Printf("Config del mes (heredada de %s): gasto %.0f%% · día corte %d · USD %.0f\n",
		cfg.HeredadaDe, cfg.PorcentajeParaGastos*100, cfg.DiaDeCorteCredito, cfg.TasaCambioUSD)
	fmt.Printf("Presupuesto para gastos (%.0f%%): $%.0f\n", cfg.PorcentajeParaGastos*100, sueldo*cfg.PorcentajeParaGastos)
	fmt.Printf("Carga de gastos actual en el mes: $%.0f\n", (sueldo*cfg.PorcentajeParaGastos)-disponible)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("DISPONIBLE RESTANTE PARA EL MES: $%.0f\n", disponible)

	if *proyectarFlag > 0 {
		fmt.Println("\n=== Proyección de Pasivos ===")
		gastos, err := adaptador.ObtenerGastosValidos(periodo)
		if err != nil {
			fmt.Printf("Error obteniendo gastos para proyectar: %v\n", err)
		} else {
			proyector := presupuesto.NewProyectorPresupuesto()
			proyecciones := proyector.Proyectar(gastos, ahora, *proyectarFlag)
			for _, p := range proyecciones {
				fmt.Printf("[%02d/%d] Total Comprometido: $%.0f\n", p.Mes, p.Anio, p.TotalComprometido)
			}
		}
		fmt.Println("=============================")
	}
}

func runSqliteSubcommand(args []string) {
	if len(args) < 1 || args[0] != "init" {
		log.Fatalf("Uso: presupuesto-cli sqlite init --db <ruta>")
	}
	fs := flag.NewFlagSet("sqlite init", flag.ExitOnError)
	dbPath := fs.String("db", "data/movimientos.db", "Ruta al archivo sqlite")
	fs.Parse(args[1:])

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("abriendo BD: %v", err)
	}
	defer db.Close()

	if err := sqlitepkg.Up(db); err != nil {
		log.Fatalf("aplicando migraciones: %v", err)
	}
	fmt.Printf("BD inicializada en %s\n", *dbPath)
}
