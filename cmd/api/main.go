package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"presupuesto/app"
)

// apiDeps agrupa las dependencias que los handlers necesitan: la app (casos de
// uso) y el caso de refresh (abstraído tras una interfaz mínima para poder
// testear el handler aislado de la ejecución real del scraper).
type apiDeps struct {
	app     *app.App
	refresh refrescador
}

func main() {
	_ = godotenv.Load()

	port := flag.String("port", "8085", "Puerto para el servidor web")
	dbPath := flag.String("db", "data/movimientos.db", "Ruta al sqlite de movimientos")
	configsPath := flag.String("configs", "data/configs-mensuales.json", "Ruta del archivo de configs mensuales")
	categoriasPath := flag.String("categorias", "data/categorias.json", "Ruta al JSON de categorías [{id,nombre,tipo}]")
	reglasPath := flag.String("reglas", "data/reglas.json", "Ruta al JSON de reglas de categorización [{patron,destino}]")
	divisionesPath := flag.String("divisiones", "data/divisiones.json", "Ruta al JSON de divisiones/overrides")
	sueldoPath := flag.String("sueldo", "data/sueldo.json", "Ruta al JSON con patrones de descripción que identifican el sueldo")
	manualesPath := flag.String("manuales", "data/manuales.json", "Ruta al JSON de gastos manuales")
	exclusionesPath := flag.String("exclusiones", "data/exclusiones.json", "Ruta al JSON con substrings de descripciones a ignorar (legacy, se migra a reglas)")
	provisorioPath := flag.String("provisorio", "data/current.json", "Ruta de salida del scraper (no se lee en runtime para servir requests)")
	flag.Parse()

	application, err := app.New(app.Config{
		ConfigsPath:     *configsPath,
		CategoriasPath:  *categoriasPath,
		ReglasPath:      *reglasPath,
		ExclusionesPath: *exclusionesPath,
		DivisionesPath:  *divisionesPath,
		SueldoPath:      *sueldoPath,
		ManualesPath:    *manualesPath,
		DBPath:          *dbPath,
		ProvisorioPath:  *provisorioPath,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close()

	deps := apiDeps{app: application, refresh: application}

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
	http.HandleFunc("/api/movimientos/categoria", deps.handleMovimientoCategoria)
	http.HandleFunc("/api/movimientos/alias", deps.handleMovimientoAlias)
	http.HandleFunc("/api/categorias", deps.handleCategorias)
	http.HandleFunc("/api/reglas", deps.handleReglas)
	http.HandleFunc("/api/configs", handlerListar(application))
	http.HandleFunc("/api/configs/", handlerSubconfigs(application))
	http.HandleFunc("/api/sueldo", deps.handleSueldo)
	http.HandleFunc("/api/exclusiones", deps.handleExclusiones)
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
