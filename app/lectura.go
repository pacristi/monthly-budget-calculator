package app

import (
	"time"

	defjson "presupuesto/definiciones/json"
	"presupuesto/movimientos/sqlite"
	"presupuesto/presupuesto"
)

// CategoriaResumen es una categoría con su presupuesto y acumulado del mes.
type CategoriaResumen struct {
	ID          string  `json:"id"`
	Nombre      string  `json:"nombre"`
	Tipo        string  `json:"tipo"`
	Porcentaje  float64 `json:"porcentaje"`
	Presupuesto float64 `json:"presupuesto"`
	Acumulado   float64 `json:"acumulado"`
}

// GastoDetalle es una línea del detalle de "Gastos en el Mes" (solo categorías
// límite con carga > 0 en el periodo).
type GastoDetalle struct {
	Fecha       string  `json:"fecha"`
	Descripcion string  `json:"descripcion"`
	Carga       float64 `json:"carga"`
	Cuotas      int     `json:"cuotas"`
	CategoriaID string  `json:"categoriaId"`
}

// Resumen es la vista del presupuesto de un mes.
type Resumen struct {
	Sueldo     float64                       `json:"sueldo"`
	Categorias []CategoriaResumen            `json:"categorias"`
	SinAsignar float64                       `json:"sinAsignar"`
	Gastos     []GastoDetalle                `json:"gastos"`
	Config     presupuesto.ConfigPresupuesto `json:"config"`
}

// Proyeccion es el total comprometido proyectado a un mes futuro.
type Proyeccion struct {
	Anio              int     `json:"anio"`
	Mes               string  `json:"mes"`
	MesNum            int     `json:"mesNum"`
	TotalComprometido float64 `json:"totalComprometido"`
}

// MovimientoVista es la vista estable de un movimiento para la UI.
type MovimientoVista struct {
	ID                  string   `json:"id"`
	Fecha               string   `json:"fecha"`
	Descripcion         string   `json:"descripcion"`
	DescripcionOriginal string   `json:"descripcionOriginal,omitempty"`
	Monto               float64  `json:"monto"`
	IsUSD               bool     `json:"isUsd"`
	MiParte             *float64 `json:"miParte,omitempty"`
	CategoriaID         string   `json:"categoriaId"`
}

var mesesNombres = []string{"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}

// periodoDeMes construye el PeriodoPresupuestario del mes de `mes`: primer día
// a las 00:00 hasta el último nanosegundo del mes.
func periodoDeMes(mes time.Time) presupuesto.PeriodoPresupuestario {
	return presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(mes.Year(), mes.Month(), 1, 0, 0, 0, 0, mes.Location()),
		Fin:    time.Date(mes.Year(), mes.Month()+1, 1, 0, 0, 0, 0, mes.Location()).Add(-time.Nanosecond),
	}
}

// idsLimite devuelve el set de ids de categorías tipo Limite (gasto). El
// detalle de gastos y la proyección de pasivos se restringen a estas: los
// aportes de meta (ahorro/inversión) no son gasto ni pasivo.
func idsLimite(cats []presupuesto.Categoria) map[string]bool {
	out := make(map[string]bool)
	for _, c := range cats {
		if c.Tipo == presupuesto.Limite {
			out[c.ID] = true
		}
	}
	return out
}

// gastosDelPeriodo trae los cargos de sqlite, los enriquece con overrides y
// reglas cacheados, y anexa los gastos manuales (que NO pasan por
// EnriquecerGastos — ver Paso 2/4). Es la base común de ResumenDelMes y
// Proyecciones.
func (a *App) gastosDelPeriodo() ([]presupuesto.Gasto, error) {
	cargos, err := sqlite.Cargos(a.db)
	if err != nil {
		return nil, err
	}
	gastos, err := presupuesto.EnriquecerGastos(cargos, a.overrides, a.reglas, a.repoConfigs)
	if err != nil {
		return nil, err
	}
	manuales, err := a.manuales.Listar()
	if err != nil {
		return nil, err
	}
	return append(gastos, manuales...), nil
}

// sueldoDelPeriodo detecta el sueldo del periodo desde los abonos de la ventana
// de búsqueda (VentanaSueldo) aplicando los patrones configurados.
func (a *App) sueldoDelPeriodo(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	patrones, err := defjson.LeerListaStrings(a.sueldoPath)
	if err != nil {
		return 0, err
	}
	ini, fin := presupuesto.VentanaSueldo(periodo)
	abonos, err := sqlite.Abonos(a.db, ini, fin)
	if err != nil {
		return 0, err
	}
	return presupuesto.DetectarSueldo(abonos, patrones, periodo)
}

// ResumenDelMes arma el desglose del presupuesto del mes: resuelve config,
// categorías, gastos enriquecidos + manuales, y sueldo, luego calcula el
// resumen y filtra el detalle de gastos a categorías límite con carga > 0.
func (a *App) ResumenDelMes(mes time.Time) (Resumen, error) {
	periodo := periodoDeMes(mes)

	cfg, err := a.repoConfigs.ParaMes(periodo.Inicio)
	if err != nil {
		return Resumen{}, err
	}

	categorias, err := a.repoCategorias.Listar()
	if err != nil {
		return Resumen{}, err
	}

	gastos, err := a.gastosDelPeriodo()
	if err != nil {
		return Resumen{}, err
	}

	sueldo, err := a.sueldoDelPeriodo(periodo)
	if err != nil {
		return Resumen{}, err
	}

	resumen, err := presupuesto.NewCalculadora(sueldo, gastos, a.repoConfigs).CalcularResumen(periodo, categorias)
	if err != nil {
		return Resumen{}, err
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

	cats := make([]CategoriaResumen, 0, len(resumen.Categorias))
	for _, c := range resumen.Categorias {
		cats = append(cats, CategoriaResumen{
			ID:          c.CategoriaID,
			Nombre:      c.Nombre,
			Tipo:        string(c.Tipo),
			Porcentaje:  cfg.Porcentajes[c.CategoriaID],
			Presupuesto: c.Presupuesto,
			Acumulado:   c.Acumulado,
		})
	}

	return Resumen{
		Sueldo:     resumen.Sueldo,
		Categorias: cats,
		SinAsignar: resumen.SinAsignar,
		Gastos:     detalles,
		Config:     cfg,
	}, nil
}

// Proyecciones proyecta el total comprometido de los gastos límite hacia
// adelante desde `desde` (tratado como "ahora" del cálculo). Los aportes de
// meta no son deuda futura, así que se excluyen.
func (a *App) Proyecciones(desde time.Time, meses int) ([]Proyeccion, error) {
	gastos, err := a.gastosDelPeriodo()
	if err != nil {
		return nil, err
	}

	categorias, err := a.repoCategorias.Listar()
	if err != nil {
		return nil, err
	}
	esLimite := idsLimite(categorias)
	gastosLimite := make([]presupuesto.Gasto, 0, len(gastos))
	for _, g := range gastos {
		if esLimite[g.CategoriaID] {
			gastosLimite = append(gastosLimite, g)
		}
	}

	proyecciones := presupuesto.NewProyectorPresupuesto().Proyectar(gastosLimite, desde, meses)

	var res []Proyeccion
	for _, p := range proyecciones {
		res = append(res, Proyeccion{
			Anio:              p.Anio,
			Mes:               mesesNombres[p.Mes-1],
			MesNum:            int(p.Mes),
			TotalComprometido: p.TotalComprometido,
		})
	}
	return res, nil
}

// Movimientos lista todos los cargos como vista de UI: aplica overrides de "mi
// parte" y clasificación (VistaMovimientos), luego el alias de descripción
// (DescripcionOverride) y mapea a MovimientoVista. Preserva el orden de la
// query de cargos (fecha ASC, id ASC).
func (a *App) Movimientos() ([]MovimientoVista, error) {
	cargos, err := sqlite.Cargos(a.db)
	if err != nil {
		return nil, err
	}
	movs := presupuesto.VistaMovimientos(cargos, a.overrides, a.reglas)

	vista := make([]MovimientoVista, 0, len(movs))
	for _, m := range movs {
		fechaISO := m.Fecha.Format("2006-01-02")
		descripcion := m.Descripcion
		descripcionOriginal := ""
		if alias := presupuesto.DescripcionOverride(m.ID, fechaISO, m.Monto, m.Descripcion, a.overrides); alias != "" {
			descripcion = alias
			descripcionOriginal = m.Descripcion
		}
		vista = append(vista, MovimientoVista{
			ID:                  m.ID,
			Fecha:               fechaISO,
			Descripcion:         descripcion,
			DescripcionOriginal: descripcionOriginal,
			Monto:               m.Monto,
			IsUSD:               m.IsUSD,
			MiParte:             m.MiParte,
			CategoriaID:         m.CategoriaID,
		})
	}
	return vista, nil
}
