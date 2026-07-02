// Command presupuesto-cli es el CLI de ingesta y utilidades de la aplicación de
// presupuesto. Delega la ingesta en el paquete ingesta (fuente + writer sqlite);
// no reimplementa lógica de negocio.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"

	"presupuesto/ingesta"
	bchile "presupuesto/ingesta/banco-de-chile"
	obcl "presupuesto/ingesta/open-banking-chile"
	sqlitepkg "presupuesto/movimientos/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("uso: %s <comando> [args]\ncomandos: sqlite-init, ingestar", os.Args[0])
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "sqlite-init":
		err = cmdSqliteInit(args)
	case "ingestar":
		err = cmdIngestar(args)
	default:
		log.Fatalf("comando desconocido: %s", cmd)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// cmdSqliteInit abre (creando si no existe) la BD sqlite y corre las migraciones.
func cmdSqliteInit(args []string) error {
	fs := flag.NewFlagSet("sqlite-init", flag.ExitOnError)
	dbPath := fs.String("db", "data/movimientos.db", "Ruta al sqlite de movimientos")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		return fmt.Errorf("abriendo BD: %w", err)
	}
	defer db.Close()
	if err := sqlitepkg.Up(db); err != nil {
		return fmt.Errorf("migraciones: %w", err)
	}
	fmt.Printf("BD inicializada en %s\n", *dbPath)
	return nil
}

// cmdIngestar despacha los subcomandos de ingesta (obchile | xlsx).
func cmdIngestar(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("uso: ingestar <obchile|xlsx> [flags]")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "obchile":
		return ingestarObchile(rest)
	case "xlsx":
		return ingestarXLSX(rest)
	default:
		return fmt.Errorf("subcomando de ingesta desconocido: %s (soporta obchile, xlsx)", sub)
	}
}

// ingestarObchile ingesta el snapshot JSON del scraper de Open Banking Chile.
func ingestarObchile(args []string) error {
	fs := flag.NewFlagSet("ingestar obchile", flag.ExitOnError)
	dbPath := fs.String("db", "data/movimientos.db", "Ruta al sqlite de movimientos")
	jsonPath := fs.String("json", "data/current.json", "Ruta al JSON del scrape")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fuente := obcl.NuevaOpenBankingChile(*jsonPath)
	return ingestarConWriter(*dbPath, "cli-obchile", fuente)
}

// ingestarXLSX ingesta cartolas .xls del Banco de Chile.
func ingestarXLSX(args []string) error {
	fs := flag.NewFlagSet("ingestar xlsx", flag.ExitOnError)
	banco := fs.String("banco", "", "Banco de la cartola (ej. bchile)")
	tipo := fs.String("tipo", "", "Tipo de cartola (cta-corriente | tc-nacional | tc-internacional)")
	anio := fs.Int("año", 0, "Año de la cartola (para cuenta corriente)")
	dir := fs.String("dir", "", "Directorio con los archivos .xls")
	dbPath := fs.String("db", "data/movimientos.db", "Ruta al sqlite de movimientos")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fuente, err := bchile.NuevaCartolaXLSX(*banco, *tipo, *anio, *dir)
	if err != nil {
		return err
	}
	return ingestarConWriter(*dbPath, "cli-xlsx", fuente)
}

// ingestarConWriter abre la BD, migra, arma el writer sqlite e ingesta la
// fuente, imprimiendo cuántos movimientos nuevos quedaron almacenados.
func ingestarConWriter(dbPath, origen string, fuente ingesta.FuenteMovimientos) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("abriendo BD: %w", err)
	}
	defer db.Close()
	if err := sqlitepkg.Up(db); err != nil {
		return fmt.Errorf("migraciones: %w", err)
	}

	writer := sqlitepkg.NewWriter(db, origen)
	nuevos, err := ingesta.Ingestar(fuente, writer)
	if err != nil {
		return err
	}
	fmt.Printf("Ingesta completa: %d movimientos nuevos\n", nuevos)
	return nil
}
