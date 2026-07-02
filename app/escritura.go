package app

import (
	"time"

	defjson "presupuesto/definiciones/json"
	"presupuesto/presupuesto"
)

// GuardarMiParte persiste el override de "mi parte" (split) de un movimiento y
// recarga el cache de overrides.
func (a *App) GuardarMiParte(o presupuesto.Override) error {
	if err := a.overridesRepo.GuardarMiParte(o); err != nil {
		return err
	}
	return a.recargarOverrides()
}

// GuardarCategoriaDeMovimiento asigna a mano la categoría de un movimiento,
// preservando el split si ya existía, y recarga el cache de overrides.
func (a *App) GuardarCategoriaDeMovimiento(o presupuesto.Override) error {
	if err := a.overridesRepo.GuardarCategoria(o); err != nil {
		return err
	}
	return a.recargarOverrides()
}

// GuardarAlias persiste el alias de descripción de un movimiento y recarga el
// cache de overrides.
func (a *App) GuardarAlias(o presupuesto.Override) error {
	if err := a.overridesRepo.GuardarAlias(o); err != nil {
		return err
	}
	return a.recargarOverrides()
}

// GuardarMonedaDeMovimiento corrige a mano si un movimiento es USD o CLP
// (la heurística automática falla en cargos USD sin decimales) y recarga el
// cache de overrides.
func (a *App) GuardarMonedaDeMovimiento(o presupuesto.Override) error {
	if err := a.overridesRepo.GuardarMoneda(o); err != nil {
		return err
	}
	return a.recargarOverrides()
}

// Categorias lista las categorías configuradas.
func (a *App) Categorias() ([]presupuesto.Categoria, error) {
	return a.repoCategorias.Listar()
}

// GuardarCategorias reemplaza el set completo de categorías.
func (a *App) GuardarCategorias(cats []presupuesto.Categoria) error {
	return a.repoCategorias.Guardar(cats)
}

// Reglas devuelve las reglas efectivas, migrando exclusiones legacy (misma
// lógica que CargarReglas). No usa el cache: refleja el estado en disco, igual
// que el handler GET /api/reglas actual.
func (a *App) Reglas() ([]presupuesto.Regla, error) {
	return defjson.CargarReglas(a.reglasPath, a.exclusionesPath)
}

// GuardarReglas reemplaza el set completo de reglas y recarga el cache en
// memoria que usan los casos de uso de lectura.
func (a *App) GuardarReglas(reglas []presupuesto.Regla) error {
	if err := defjson.EscribirReglas(a.reglasPath, reglas); err != nil {
		return err
	}
	nuevas, err := defjson.CargarReglas(a.reglasPath, a.exclusionesPath)
	if err != nil {
		return err
	}
	a.reglas = nuevas
	return nil
}

// Configs lista las configuraciones mensuales declaradas.
func (a *App) Configs() ([]defjson.ConfigMensual, error) {
	return a.repoConfigs.Listar()
}

// ConfigResuelta devuelve la configuración efectiva para un mes.
func (a *App) ConfigResuelta(mes time.Time) (presupuesto.ConfigPresupuesto, error) {
	return a.repoConfigs.ParaMes(mes)
}

// GuardarConfig persiste (crea o reemplaza) una configuración mensual.
func (a *App) GuardarConfig(c defjson.ConfigMensual) error {
	return a.repoConfigs.Guardar(c)
}

// BorrarConfig elimina la declaración de configuración de un mes.
func (a *App) BorrarConfig(mesDesde string) error {
	return a.repoConfigs.Borrar(mesDesde)
}

// PatronesSueldo lista los patrones de descripción que identifican el sueldo.
func (a *App) PatronesSueldo() ([]string, error) {
	return defjson.LeerListaStrings(a.sueldoPath)
}

// GuardarPatronesSueldo reemplaza el set completo de patrones de sueldo.
func (a *App) GuardarPatronesSueldo(patrones []string) error {
	return defjson.EscribirListaStrings(a.sueldoPath, patrones)
}

// Exclusiones lista las exclusiones legacy (substrings a ignorar), mientras
// exista la migración a reglas.
func (a *App) Exclusiones() ([]string, error) {
	return defjson.LeerListaStrings(a.exclusionesPath)
}

// GuardarExclusiones reemplaza el set completo de exclusiones legacy.
func (a *App) GuardarExclusiones(exclusiones []string) error {
	return defjson.EscribirListaStrings(a.exclusionesPath, exclusiones)
}
