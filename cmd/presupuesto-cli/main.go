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
	"presupuesto/internal/app/bootstrap"
	"presupuesto/internal/cartola/fuentes"
	"presupuesto/internal/cartola/ingesta"
	sqlitepkg "presupuesto/internal/cartola/sqlite"
	"presupuesto/internal/presupuesto"
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
	proveedorFlag := flag.String("proveedor", "compuesto", "Fuente: compuesto (sqlite liquidado + scrape provisorio, default) | sqlite (solo liquidado) | obchile (legacy)")
	dbPathFlag := flag.String("db", "data/movimientos.db", "Ruta al sqlite (modos compuesto/sqlite)")
	provisorioFlag := flag.String("provisorio", "data/current.json", "Ruta al JSON del último scrape (capa provisoria; modo compuesto)")
	divisionesFlag := flag.String("divisiones", "", "Ruta al JSON de divisiones (modos compuesto/sqlite; en obchile es posicional)")
	exclusionesFlag := flag.String("exclusiones", "data/exclusiones.json", "Ruta al JSON con substrings de descripciones a ignorar (legacy, se migra a reglas)")
	reglasFlag := flag.String("reglas", "data/reglas.json", "Ruta al JSON de reglas de categorización [{patron,destino}]")
	categoriasFlag := flag.String("categorias", "data/categorias.json", "Ruta al JSON de categorías [{id,nombre,tipo}]")
	sueldoFlag := flag.String("sueldo", "data/sueldo.json", "Ruta al JSON con patrones de descripción que identifican el sueldo")
	manualesFlag := flag.String("manuales", "data/manuales.json", "Ruta al JSON de gastos manuales")
	flag.Parse()

	fmt.Printf("Iniciando Calculadora de Presupuesto Mensual\n")

	legacyJSONPath := ""
	divisionesPath := *divisionesFlag
	if *proveedorFlag == "obchile" {
		args := flag.Args()
		if len(args) < 1 {
			log.Fatalf("Uso: presupuesto-cli [...] --proveedor obchile <ruta_json> [ruta_divisiones]")
		}
		legacyJSONPath = args[0]
		divisionesPath = ""
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

	switch app.Proveedor {
	case "obchile":
		fmt.Printf("Leyendo desde JSON: %s\n", legacyJSONPath)
	case "sqlite":
		fmt.Printf("Leyendo desde sqlite: %s\n", *dbPathFlag)
	case "compuesto":
		fmt.Printf("Leyendo compuesto: sqlite %s + provisorio %s\n", *dbPathFlag, app.ProvisorioPath)
	}

	adaptador := app.Adaptador
	repoConfigs := app.RepoConfigs
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

	categorias, err := app.RepoCategorias.Listar()
	if err != nil {
		log.Fatalf("Error leyendo categorías: %v", err)
	}

	resumen, err := calc.CalcularResumen(periodo, categorias)
	if err != nil {
		log.Fatalf("Error calculando presupuesto: %v", err)
	}

	fmt.Printf("Sueldo detectado: $%.0f\n", resumen.Sueldo)

	if *detalleFlag {
		fmt.Println("\n--- Detalle de Gastos Imputados este Mes ---")
		gastos, _ := adaptador.ObtenerGastosValidos(periodo)
		for _, g := range gastos {
			carga := g.CalcularCargaParaPeriodo(periodo)
			if carga > 0 {
				fmt.Printf("[%s] %s (%s) | Monto a pagar: $%.0f (Dividido en %d cuotas)\n", g.FechaTransaccion.Format("02/01/2006"), g.Descripcion, g.CategoriaID, carga, g.Cuotas)
			}
		}
		fmt.Println("--------------------------------------------")
	}

	fmt.Printf("Config del mes (heredada de %s): día corte %d · USD %.0f\n",
		cfg.HeredadaDe, cfg.DiaDeCorteCredito, cfg.TasaCambioUSD)
	fmt.Println("--------------------------------------------------")
	for _, c := range resumen.Categorias {
		restante := c.Presupuesto - c.Acumulado
		switch c.Tipo {
		case presupuesto.Meta:
			fmt.Printf("[META]   %-12s meta $%.0f · aportado $%.0f · faltan $%.0f\n", c.Nombre, c.Presupuesto, c.Acumulado, restante)
		default:
			fmt.Printf("[LÍMITE] %-12s tope $%.0f · gastado $%.0f · quedan $%.0f\n", c.Nombre, c.Presupuesto, c.Acumulado, restante)
		}
	}
	if resumen.SinAsignar != 0 {
		fmt.Printf("Sin asignar: $%.0f\n", resumen.SinAsignar)
	}

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

	repo, cerrar := abrirRepoMovimientos(*dbPath, "obchile")
	defer cerrar()

	n, err := ingesta.DesdeFuente(fuentes.NuevaOpenBankingChile(*jsonPath), repo)
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

	fuente, err := fuentes.NuevaCartolaXLSX(*banco, *tipo, *año, *dir)
	if err != nil {
		log.Fatalf("seleccionando fuente: %v", err)
	}

	repo, cerrar := abrirRepoMovimientos(*dbPath, "xlsx")
	defer cerrar()

	n, err := ingesta.DesdeFuente(fuente, repo)
	if err != nil {
		log.Fatalf("ingesta xlsx: %v", err)
	}
	fmt.Printf("Ingesta xlsx: %d movimientos nuevos\n", n)
}

func abrirRepoMovimientos(dbPath, origen string) (*sqlitepkg.Writer, func()) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("abriendo BD: %v", err)
	}
	if err := sqlitepkg.Up(db); err != nil {
		db.Close()
		log.Fatalf("migraciones: %v", err)
	}
	return sqlitepkg.NewWriter(db, origen), func() { db.Close() }
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
