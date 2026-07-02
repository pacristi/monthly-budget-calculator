# Calculadora de Presupuesto Mensual 💸

Una herramienta que te dice **cuánta plata tienes disponible este mes**, considerando tu sueldo, tus gastos del mes, y las cuotas futuras ya comprometidas.

Corre **localmente en tu computador**. Tu data nunca sale de tu máquina.

---

## Quick start (3 comandos)

Si tienes Go y Node instalados:

```bash
git clone https://github.com/pacristi/monthly-budget-calculator
cd monthly-budget-calculator
make start
```

`make start` te va a hacer un par de preguntas (banco, RUT, clave), traer tu cartola del día, y abrir el dashboard en `http://localhost:8085`.

**Eso es todo.** No necesitas más para usarlo día a día.

---

## ¿Qué hace el dashboard?

- 💰 **Sueldo detectado** del mes.
- 🎯 **Categorías que reparten tu sueldo** (ej. 25% gasto, 50% inversión, 25% ahorro), cada una con su % configurable por mes.
- 📊 **Barras por categoría**: las de tipo *meta* (ahorro/inversión) verdean al llenarse — llegar al 100% es bueno; las de tipo *límite* (gasto) enrojecen al acercarse al tope.
- 📋 **Tabla de movimientos** con selector de categoría por fila y dos botones:
  - **Mi parte** — para gastos compartidos (asado donde solo pusiste $20.000 de $80.000).
  - **No contar** — para gastos que te van a devolver o que no son tuyos.
- 🔮 **Proyección** de los próximos meses (cuotas comprometidas; solo categorías de tipo límite).
- ⚙️ **Pestaña Configuración** para editar categorías, reglas de categorización, patrones de sueldo y configs mensuales desde la UI.

## Uso diario

```bash
make ingest    # trae la cartola de hoy
make serve     # abre el dashboard
```

O pones `make ingest` en un cron y dejas `make serve` corriendo en background.

## Bancos soportados

Cualquier banco soportado por [open-banking-chile](https://www.npmjs.com/package/open-banking-chile):

- Banco de Chile (`bchile`) — usado y testeado por el autor.
- Banco Estado (`banco_estado`).
- Santander (`santander`).
- BCI (`bci`).
- (y otros — ver el paquete)

Lo configuras con `BANCO_ID=...` en tu `.env`. El wizard `make start` lo pregunta la primera vez.

## Configuración

El wizard te crea los archivos por defecto, pero puedes ajustarlos:

| Archivo | Qué controla |
|---|---|
| `.env` | Credenciales y banco |
| `data/sueldo.json` | Substrings que identifican tu depósito de sueldo (ej. `["pago de sueldos"]`) |
| `data/categorias.json` | Categorías globales `[{id,nombre,tipo}]` (tipo: `meta` o `limite`) — se administra desde la UI |
| `data/reglas.json` | Reglas `[{patron,destino}]` que mandan un movimiento a una categoría o a `ignorado` — se administra desde la UI |
| `data/exclusiones.json` | Legacy: substrings a ignorar. Se migra automáticamente a reglas con destino `ignorado` |
| `data/divisiones.json` | Overrides por movimiento: "mi parte" y/o categoría asignada a mano — se administra desde la UI |
| `data/configs-mensuales.json` | % por categoría, día de corte de la TC, tasa USD — se administra desde la UI |

**Todo es editable desde la pestaña Configuración del dashboard.** No tienes que tocar JSONs a mano.

---

## Histórico con cartolas `.xls`

El scraper diario (`make ingest`) trae típicamente solo movimientos del mes en curso. Toda la data se persiste en una BD local (sqlite) — el scraper no es la única fuente. Si además quieres cargar histórico (para ver presupuestos de meses pasados, o para que las cuotas comprometidas se sigan proyectando aunque el scraper ya no las traiga), puedes ingestar las cartolas `.xls` que descargas de la web del banco hacia esa misma BD. **No es necesario para uso normal.**

### Cómo

1. **Inicializar la BD** (si aún no existe):

```bash
make sqlite-init
```

2. **Cargar histórico** desde cartolas `.xls`. Una corrida por (año × tipo de cuenta):

```bash
make ingest-xlsx-cta-corriente AÑO=2025 DIR="/path/a/Cuenta Corriente/2025"
make ingest-xlsx-tc-nacional   DIR="/path/a/Tarjeta de Credito/Nacional/2025"
make ingest-xlsx-tc-internacional DIR="/path/a/Tarjeta de Credito/Internacional/2025"
```

(Idempotente: si lo corres dos veces, no duplica. Los `.xls` van a la misma BD que el scraper; `make serve` la sirve igual.)

### Limitaciones

- Solo cartolas de **Banco de Chile** tienen parsers `.xls`. Otros bancos requieren implementar un parser que produzca `movimientos.MovimientoBruto`.
- El scraper de Open Banking sí funciona con varios bancos.

---

## Cómo extender a otro banco con histórico `.xls`

Solo si quieres cargar cartolas históricas de otro banco. Pasos:

1. Crear un paquete `ingesta/<banco>/parser` con la lógica pura que traduce la cartola a `movimientos.MovimientoBruto`.
2. Mirar `ingesta/banco-de-chile/parser/cta_corriente.go` como referencia. El patrón:
   - Capa I/O que abre el `.xls` con `extrame/xls`.
   - Capa pura que transforma filas crudas a `MovimientoBruto`.
3. Exponer una fuente en `ingesta/<banco>/` que implemente `ingesta.FuenteMovimientos` (ver `ingesta/banco-de-chile/fuente.go`).

Flujo de ingesta: `ingesta/<banco>/parser` parsea y normaliza datos externos, el runner `ingesta.Ingestar` orquesta lectura y guardado, y `movimientos/sqlite` persiste con deduplicación.

PRs bienvenidas.

---

## Troubleshooting

**"Sueldo no encontrado"**: edita `data/sueldo.json` (o la pestaña Configuración) con la substring que aparece en tu depósito de sueldo. Ej: si tu empleador escribe `"REMUNERACION MAYO"`, agregas `remuneracion`.

**"Los montos están raros"**: si pasaste por varias versiones del software, puedes tener data corrupta. `rm data/movimientos.db && make sqlite-init` y vuelves a ingestar (scraper y/o `.xls`).

**"No puedo guardar mi parte / 400 Bad Request"**: asegúrate de levantar el API con `data/divisiones.json` accesible. `make serve` ya lo pasa correctamente.

---

## Licencia

Proyecto personal de [pacristi](https://github.com/pacristi). Si te sirve y tu banco no está, manda un PR. 🙂
