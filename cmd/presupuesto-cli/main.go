package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
	obchileingest "github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest/obchile"
	xlsxpkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest/xlsx"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/obchile"
	sqlitepkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/sqlite"
	"github.com/pierocristi/monthly-budget-calculator/internal/config"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "sqlite" {
		runSqliteSubcommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "ingestar" {
		runIngestarSubcommand(os.Args[2:])
		return
	}

	_ = godotenv.Load()

	detalleFlag := flag.Bool("detalle", false, "Mostrar la lista de gastos que impactan este mes")
	proyectarFlag := flag.Int("proyectar", 0, "Proyectar la carga de gastos para los próximos N meses")
	rutaConfigsFlag := flag.String("configs", "data/configs-mensuales.json", "Ruta del archivo de configs mensuales")
	proveedorFlag := flag.String("proveedor", "obchile", "Fuente: obchile (JSON del scraper, default) | sqlite")
	dbPathFlag := flag.String("db", "data/movimientos.db", "Ruta al sqlite (solo si --proveedor=sqlite)")
	divisionesFlag := flag.String("divisiones", "", "Ruta al JSON de divisiones (solo si --proveedor=sqlite; en obchile es posicional)")
	exclusionesFlag := flag.String("exclusiones", "data/exclusiones.json", "Ruta al JSON con substrings de descripciones a ignorar (ahorros, traspasos, pagos TC)")
	sueldoFlag := flag.String("sueldo", "data/sueldo.json", "Ruta al JSON con patrones de descripción que identifican el sueldo")
	manualesFlag := flag.String("manuales", "data/manuales.json", "Ruta al JSON de gastos manuales")
	flag.Parse()

	fmt.Printf("Iniciando Calculadora de Presupuesto Mensual\n")

	repoConfigs := config.NewRepoJSON(*rutaConfigsFlag)
	if err := config.EnsureSeed(repoConfigs, config.SeedPorDefecto(time.Now())); err != nil {
		log.Fatalf("inicializando configs: %v", err)
	}

	var adaptador presupuesto.ProveedorFinanciero

	switch *proveedorFlag {
	case "obchile":
		args := flag.Args()
		if len(args) < 1 {
			log.Fatalf("Uso: presupuesto-cli [...] --proveedor obchile <ruta_json> [ruta_divisiones]")
		}
		rutaJson := args[0]
		rutaDivisiones := ""
		if len(args) > 1 {
			rutaDivisiones = args[1]
		}
		fmt.Printf("Leyendo desde JSON: %s\n", rutaJson)
		adaptador = obchile.NewAdapter(rutaJson, rutaDivisiones, *exclusionesFlag, *sueldoFlag, *manualesFlag, repoConfigs)
	case "sqlite":
		db, err := sql.Open("sqlite", *dbPathFlag)
		if err != nil {
			log.Fatalf("abriendo BD: %v", err)
		}
		defer db.Close()
		if err := sqlitepkg.Up(db); err != nil {
			log.Fatalf("migraciones: %v", err)
		}
		fmt.Printf("Leyendo desde sqlite: %s\n", *dbPathFlag)
		adaptador = sqlitepkg.NewAdapter(db, *divisionesFlag, *exclusionesFlag, *sueldoFlag, *manualesFlag, repoConfigs)
	default:
		log.Fatalf("--proveedor inválido: %s (obchile | sqlite)", *proveedorFlag)
	}
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

func runIngestarSubcommand(args []string) {
	if len(args) < 1 {
		log.Fatalf("Uso: presupuesto-cli ingestar {obchile|xlsx} ...")
	}
	switch args[0] {
	case "obchile":
		runIngestarObchile(args[1:])
	case "xlsx":
		runIngestarXlsx(args[1:])
	default:
		log.Fatalf("Subcomando ingestar desconocido: %s. Usos: obchile", args[0])
	}
}

func runIngestarObchile(args []string) {
	fs := flag.NewFlagSet("ingestar obchile", flag.ExitOnError)
	dbPath := fs.String("db", "data/movimientos.db", "Ruta al archivo sqlite")
	jsonPath := fs.String("json", "data/current.json", "Ruta al JSON producido por el scraper")
	fs.Parse(args)

	n, err := obchileingest.Ingestar(*jsonPath, *dbPath)
	if err != nil {
		log.Fatalf("ingesta obchile: %v", err)
	}
	fmt.Printf("Ingesta obchile: %d movimientos nuevos\n", n)
}

func runIngestarXlsx(args []string) {
	fs := flag.NewFlagSet("ingestar xlsx", flag.ExitOnError)
	dbPath := fs.String("db", "data/movimientos.db", "Ruta al archivo sqlite")
	banco := fs.String("banco", "", "Banco (ej: bchile)")
	tipo := fs.String("tipo", "", "Tipo de cuenta: cta-corriente | tc-nacional | tc-internacional")
	año := fs.Int("año", 0, "Año de la cartola (obligatorio para cta-corriente)")
	dir := fs.String("dir", "", "Directorio con los archivos .xls")
	fs.Parse(args)

	if *banco == "" || *tipo == "" || *dir == "" {
		log.Fatalf("Uso: presupuesto-cli ingestar xlsx --banco bchile --tipo cta-corriente --año 2025 --dir <ruta>")
	}

	parser, err := elegirParserXlsx(*banco, *tipo)
	if err != nil {
		log.Fatalf("seleccionando parser: %v", err)
	}

	archivos, err := filepath.Glob(filepath.Join(*dir, "*.xls"))
	if err != nil {
		log.Fatalf("listando %s: %v", *dir, err)
	}
	if len(archivos) == 0 {
		log.Fatalf("ningún .xls en %s", *dir)
	}

	var batch []ingest.MovimientoBruto
	for _, a := range archivos {
		movs, err := parser.Parsear(a, *año)
		if err != nil {
			log.Fatalf("parseando %s: %v", a, err)
		}
		fmt.Printf("  • %s: %d movimientos\n", filepath.Base(a), len(movs))
		batch = append(batch, movs...)
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("abriendo BD: %v", err)
	}
	defer db.Close()

	if err := sqlitepkg.Up(db); err != nil {
		log.Fatalf("migraciones: %v", err)
	}

	writer := sqlitepkg.NewWriter(db, "xlsx")
	n, err := writer.InsertarConDedup(batch)
	if err != nil {
		log.Fatalf("insert: %v", err)
	}
	fmt.Printf("Ingesta xlsx: %d movimientos nuevos\n", n)
}

func elegirParserXlsx(banco, tipo string) (xlsxpkg.ParserCartolaXLSX, error) {
	if banco != "bchile" {
		return nil, fmt.Errorf("banco no soportado: %s (solo 'bchile' por ahora)", banco)
	}
	switch tipo {
	case "cta-corriente":
		return xlsxpkg.NewBchileCuentaCorriente(), nil
	case "tc-nacional":
		return xlsxpkg.NewBchileTCNacional(), nil
	case "tc-internacional":
		return xlsxpkg.NewBchileTCInternacional(), nil
	default:
		return nil, fmt.Errorf("tipo no soportado en esta versión: %s (soporta cta-corriente, tc-nacional, tc-internacional)", tipo)
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
