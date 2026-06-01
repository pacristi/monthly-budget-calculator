# Calculadora de Presupuesto Mensual 💸

Una herramienta que te dice **cuánta plata tenés disponible este mes**, considerando tu sueldo, tus gastos del mes, y las cuotas futuras ya comprometidas.

Corre **localmente en tu computador**. Tu data nunca sale de tu máquina.

---

## Quick start (3 comandos)

Si tenés Go y Node instalados:

```bash
git clone https://github.com/pacristi/monthly-budget-calculator
cd monthly-budget-calculator
make start
```

`make start` te va a hacer un par de preguntas (banco, RUT, clave), traer tu cartola del día, y abrirte el dashboard en `http://localhost:8085`.

**Eso es todo.** No necesitás más para usarlo día a día.

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

O ponés `make ingest` en un cron y `make serve` corriendo en background.

## Bancos soportados

Cualquier banco soportado por [open-banking-chile](https://www.npmjs.com/package/open-banking-chile):

- Banco de Chile (`bchile`) — usado y testeado por el autor.
- Banco Estado (`banco_estado`).
- Santander (`santander`).
- BCI (`bci`).
- (y otros — ver el paquete)

Lo configurás con `BANCO_ID=...` en tu `.env`. El wizard `make start` lo pregunta la primera vez.

## Configuración

El wizard te crea los archivos por defecto, pero podés ajustarlos:

| Archivo | Qué controla |
|---|---|
| `.env` | Credenciales y banco |
| `data/sueldo.json` | Substrings que identifican tu depósito de sueldo (ej. `["pago de sueldos"]`) |
| `data/categorias.json` | Categorías globales `[{id,nombre,tipo}]` (tipo: `meta` o `limite`) — se administra desde la UI |
| `data/reglas.json` | Reglas `[{patron,destino}]` que mandan un movimiento a una categoría o a `ignorado` — se administra desde la UI |
| `data/exclusiones.json` | Legacy: substrings a ignorar. Se migra automáticamente a reglas con destino `ignorado` |
| `data/divisiones.json` | Overrides por movimiento: "mi parte" y/o categoría asignada a mano — se administra desde la UI |
| `data/configs-mensuales.json` | % por categoría, día de corte de la TC, tasa USD — se administra desde la UI |

**Todo es editable desde la pestaña Configuración del dashboard.** No tenés que tocar JSONs a mano.

---

## ¿Cuándo necesito algo más?

El modo simple usa el **scraper diario**, que típicamente trae solo movimientos del mes en curso. Si querés:

- Ver presupuestos de meses pasados.
- Cargar histórico desde cartolas `.xls` que descargás del banco.
- Tener tu data persistida localmente sin depender del scraper.

...entonces te interesa el **modo avanzado** abajo. **No es necesario para uso normal.**

---

## Avanzado: histórico con cartolas `.xls`

### Cuándo

- Querés ver el presupuesto de hace 6 meses.
- Querés que las cuotas comprometidas en los últimos meses se sigan proyectando aunque el scraper ya no las traiga.
- Querés persistir tu data en una BD local (sqlite) en vez de re-scrapear cada día.

### Cómo

1. **Inicializar la BD:**

```bash
make sqlite-init
```

2. **Cargar histórico** desde cartolas `.xls` (las que descargás de la web del banco). Una corrida por (año × tipo de cuenta):

```bash
make ingest-xlsx-cta-corriente AÑO=2025 DIR="/path/a/Cuenta Corriente/2025"
make ingest-xlsx-tc-nacional   DIR="/path/a/Tarjeta de Credito/Nacional/2025"
make ingest-xlsx-tc-internacional DIR="/path/a/Tarjeta de Credito/Internacional/2025"
```

(Idempotente: si lo corrés dos veces, no duplica.)

3. **Cambiar al modo sqlite:**

```bash
make ingest-sqlite   # scraper + volcado al sqlite (reemplaza `make ingest`)
```

Y para el dashboard / cálculo:

```bash
go run ./cmd/presupuesto-api --proveedor sqlite --db data/movimientos.db --divisiones data/divisiones.json
```

### Limitaciones

- Solo cartolas de **Banco de Chile** tienen parsers `.xls`. Otros bancos requieren implementar un parser (interfaz `ParserCartolaXLSX` en `internal/cartola/ingest/xlsx/`).
- El scraper de Open Banking sí funciona con varios bancos en modo simple.

---

## Cómo extender a otro banco con histórico `.xls`

Solo si querés cargar cartolas históricas de otro banco. Pasos:

1. Crear `internal/cartola/ingest/xlsx/<banco>_<tipo>.go` implementando la interfaz `ParserCartolaXLSX`.
2. Mirar `bchile_cta_corriente.go` como referencia. El patrón:
   - Capa I/O que abre el `.xls` con `extrame/xls`.
   - Capa pura que transforma filas crudas a `MovimientoBruto`.
3. Registrar el parser en el switch de `cmd/presupuesto-cli/main.go:elegirParserXlsx`.

PRs bienvenidas.

---

## Troubleshooting

**"Sueldo no encontrado"**: editá `data/sueldo.json` (o la pestaña Configuración) con la substring que aparece en tu depósito de sueldo. Ej: si tu empleador escribe `"REMUNERACION MAYO"`, agregás `remuneracion`.

**"Los montos están raros"**: si pasaste por varias versiones del software, podés tener data corrupta. En modo simple no pasa (el scraper sobrescribe `data/current.json` cada vez). En modo avanzado, `rm data/movimientos.db && make sqlite-init` y volvés a cargar.

**"No puedo guardar mi parte / 400 Bad Request"**: asegurate de levantar el API con `data/divisiones.json` accesible. En modo simple, `make serve` ya lo pasa correctamente.

---

## Licencia

Proyecto personal de [pacristi](https://github.com/pacristi). Si te sirve y tu banco no está, mandá un PR. 🙂
