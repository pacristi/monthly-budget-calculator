# Calculadora de Presupuesto Mensual 💸

Una herramienta personal que te dice **cuánta plata tenés disponible para el mes**, considerando tu sueldo, tus gastos pasados, y las cuotas futuras que ya tenés comprometidas.

No es una app SaaS, no es un servicio en la nube. Es un proyecto que corre **localmente en tu computador**, lee tus cartolas, y te muestra un dashboard simple. Tu data no sale de tu máquina.

## ¿Para quién es esto?

Para alguien que:

- Tiene cuenta corriente y/o tarjeta(s) de crédito.
- Quiere ver mes a mes cuánto le queda disponible después de gastos y cuotas.
- Quiere poder marcar gastos compartidos (ej: cuenta de luz dividida con la pareja, asado con amigos donde solo pongo $20.000).
- Está cómodo abriendo una terminal y corriendo un par de comandos.

Si te suena, seguí leyendo.

## ¿Qué hace?

1. **Lee tus movimientos** desde dos fuentes:
   - Un scraper automático (hoy soporta Banco de Chile vía [open-banking-chile](https://www.npmjs.com/package/open-banking-chile)).
   - Cartolas históricas que descargás del banco en formato `.xls`.
2. Los guarda en una BD local sqlite (un solo archivo `data/movimientos.db`).
3. Te muestra un dashboard web con:
   - **Sueldo** del mes.
   - **Presupuesto disponible** para gastos (un % configurable del sueldo).
   - **Gastos del mes** ya cargados (incluyendo cuotas futuras de compras anteriores).
   - **Disponible restante**.
   - **Tabla de movimientos** con dos botones por fila: "editar mi parte" y "no contar".
   - **Proyección** de los próximos N meses (cuotas comprometidas).

## Bancos y proveedores soportados

| Banco | Cuenta corriente | TC Nacional | TC Internacional | Scraper diario |
|---|---|---|---|---|
| Banco de Chile | ✅ | ✅ | ✅ | ✅ (vía OBChile) |

¿No está tu banco? Mirá la sección [Cómo extender](#cómo-extender-a-otro-banco).

## Setup (primera vez)

### Prerequisitos

- Go ≥ 1.25
- Node.js (solo si vas a usar el scraper diario)
- Git

### 1. Clonar e instalar

```bash
git clone https://github.com/pacristi/monthly-budget-calculator
cd monthly-budget-calculator
make setup
```

Esto crea las carpetas necesarias, instala deps del scraper Node y resuelve los módulos Go.

### 2. Configurar tu config mensual

Editá `data/configs-mensuales.json` (te lo crea `make setup` vacío). Ejemplo:

```json
[
  {
    "mesDesde": "2025-01",
    "porcentajeParaGastos": 0.5,
    "diaDeCorteCredito": 22,
    "tasaCambioUSD": 950
  }
]
```

- `porcentajeParaGastos`: qué fracción de tu sueldo destinás a gastos. Si ganás $1M y querés tener un presupuesto de $500.000, ponés `0.5`.
- `diaDeCorteCredito`: el día del mes en que tu banco cierra el período de tu TC.
- `tasaCambioUSD`: para convertir gastos en USD (la TC internacional) a CLP.

### 3. Configurar credenciales del scraper

Si vas a usar el scraper diario, creá un `.env` en la raíz:

```env
BANCO_RUT=20.430.095-K
BANCO_PASS=tu_clave_internet
```

### 4. Configurar exclusiones (opcional pero recomendado)

Hay descripciones que el sistema **debería ignorar** porque no son gastos reales (traspasos a tu propia cuenta de ahorro, pagos de la tarjeta de crédito desde tu cuenta corriente, etc.).

```bash
cp data/exclusiones.example.json data/exclusiones.json
# editar y poner substrings tuyas
```

Cada string es una substring (case-insensitive) que si aparece en la descripción, el movimiento se ignora. Ejemplos:

```json
["pago tarjeta de credito", "fintual", "cargo por pago tc"]
```

### 5. Configurar patrones del sueldo

El sistema necesita reconocer qué movimiento es tu sueldo. Cada empleador escribe la descripción a su gusto: a vos puede llegarte como `"PAGO DE SUELDOS [RUT]"`, a otro como `"REMUNERACION MAYO 2026"`, etc.

```bash
cp data/sueldo.example.json data/sueldo.json
# editar y poner los patrones que usa tu empleador
```

Cualquier substring que aparezca en la descripción del depósito sirve. Si tu empleador escribe `"REMUNERACION"`, ponés `["remuneracion"]`. Sin esto el sistema no detecta tu sueldo y el presupuesto no se calcula.

### 6. Inicializar la BD

```bash
go run ./cmd/presupuesto-cli sqlite init --db data/movimientos.db
```

## Cargar tu histórico

El scraper diario solo trae movimientos del mes corriente. Para tener histórico, descargás tus cartolas `.xls` desde la web de tu banco y las cargás una vez.

**Convención**: un directorio = una `(banco, tipo de cuenta, año)`.

Ej. para Banco de Chile, descargás todos tus `.xls` de 2025 de cuenta corriente y los pones en una carpeta. Después:

```bash
go run ./cmd/presupuesto-cli ingestar xlsx \
  --banco bchile \
  --tipo cta-corriente \
  --año 2025 \
  --dir "/ruta/a/Cuenta Corriente/2025"
```

Y lo mismo por cada año × tipo:

```bash
# TC nacional 2025
... --tipo tc-nacional --dir ".../Tarjeta de Credito/nacional/2025"

# TC internacional 2025
... --tipo tc-internacional --dir ".../Tarjeta de Credito/Internacional/2025"

# Y repetir para 2026, etc.
```

Es **idempotente**: si corrés el mismo comando dos veces, no duplica nada.

## Uso diario

### Trae cartola de hoy

```bash
make ingest
```

Esto corre el scraper y vuelca al sqlite. Idempotente.

### Ver el presupuesto del mes (CLI)

```bash
go run ./cmd/presupuesto-cli \
  --proveedor sqlite \
  --db data/movimientos.db \
  --divisiones data/divisiones.json \
  --exclusiones data/exclusiones.json \
  --sueldo data/sueldo.json
```

### Abrir el dashboard web

```bash
go run ./cmd/presupuesto-api \
  --proveedor sqlite \
  --db data/movimientos.db \
  --divisiones data/divisiones.json \
  --exclusiones data/exclusiones.json \
  --sueldo data/sueldo.json
```

Y abrís `http://localhost:8085` en tu navegador.

### "Editar mi parte" / "No contar"

En la tabla de movimientos vas a ver dos botones por fila:

- **Editar mi parte**: para gastos compartidos. Si un asado costó $50.000 y solo pusiste $20.000, abrís el modal y ponés $20.000 como "mi parte". El presupuesto solo descuenta $20.000.
- **No contar**: atajo para marcar un gasto como `mi parte = 0`. Útil cuando un amigo te paga después o cuando el cargo te lo van a devolver.

Estos se persisten en `data/divisiones.json`. Es solo tuyo, no toca la BD de movimientos.

## Cómo extender a otro banco

Si tu banco no es Banco de Chile, podés agregarlo:

### Para cartolas .xls

Implementás la interfaz `ParserCartolaXLSX`:

```go
type ParserCartolaXLSX interface {
    Banco() string
    Source() string
    Parsear(path string, año int) ([]ingest.MovimientoBruto, error)
}
```

Mirá `internal/cartola/ingest/xlsx/bchile_cta_corriente.go` como referencia. El patrón es:

1. Una capa de I/O que abre el `.xls` con `extrame/xls` y extrae filas crudas.
2. Una capa pura que transforma las filas en `MovimientoBruto`, aplicando filtros y normalizando signos.

Después agregás tu parser al switch en `cmd/presupuesto-cli/main.go:elegirParserXlsx`.

### Para un scraper distinto

Si tenés otro scraper (BancoEstado, Santander, etc.) que genere un JSON con movimientos, podés:

1. Adaptar `internal/cartola/ingest/obchile/ingestor.go` o crear un paquete análogo.
2. Mapear cada movimiento a `MovimientoBruto`.
3. Agregar un subcomando `presupuesto-cli ingestar <tu-scraper> --json ...`.

## Arquitectura (en una imagen mental)

```
┌─────────────────┐     ┌──────────────────┐
│  Scraper (Node) │────▶│  current.json    │
│  (OBChile, etc) │     └──────────────────┘
└─────────────────┘              │
                                 ▼
┌─────────────────┐     ┌──────────────────────────────┐     ┌──────────────────┐
│  Cartolas .xls  │────▶│  sqlite/movimientos.db       │◀────│ divisiones.json  │
│  (histórico)    │     │  (raw layer + dedup)         │     │ exclusiones.json │
└─────────────────┘     └──────────────────────────────┘     └──────────────────┘
                                 │                                    │
                                 │                                    ▼
                                 │                          (overrides personales)
                                 ▼
                       ┌──────────────────────┐
                       │  Adapter sqlite      │ ───▶ presupuesto.Gasto[]
                       │  (filtros, signos,   │      │
                       │   cuotas)            │      ▼
                       └──────────────────────┘    Calculadora ──▶ "te quedan $X"
```

- **Raw layer**: el sqlite guarda movimientos como los entrega el banco, sin opinión.
- **Adapter**: aplica overrides, exclusiones y normalización USD → CLP.
- **Calculadora**: distribuye cuotas y suma cargas del mes.

## Estructura de archivos personales

| Archivo | Qué contiene | Quién lo gestiona |
|---|---|---|
| `data/movimientos.db` | Movimientos crudos | Scraper + ingestar xlsx |
| `data/divisiones.json` | Overrides "mi parte" | Vos (via UI o a mano) |
| `data/exclusiones.json` | Substrings a ignorar | Vos |
| `data/sueldo.json` | Patrones que identifican tu sueldo | Vos |
| `data/configs-mensuales.json` | Config por mes | Vos |
| `data/manuales.json` | Gastos manuales | Vos (a mano) |

Todos están en `.gitignore`. Solo el repo tiene los `.example.json`.

## Problemas comunes

**"Sueldo no encontrado en periodo"**: si es el primer día del mes y todavía no te depositaron, el sistema usa el sueldo del mes anterior como estimación automáticamente. Si tampoco hay registro de mes anterior (BD recién inicializada), tenés que cargar al menos un mes de histórico vía xlsx.

**"Los montos están raros / inflados"**: probablemente tu BD se cargó con una versión vieja del software. Borrá la BD y volvé a cargar:
```bash
rm data/movimientos.db
go run ./cmd/presupuesto-cli sqlite init --db data/movimientos.db
# y volver a correr los `ingestar xlsx` y `make ingest`
```
Tus overrides y configs no se tocan, solo los movimientos.

**"El editar mi parte no guarda"**: asegurate de levantar el API con `--divisiones data/divisiones.json`. Si la ruta no está, el API devuelve 400.

## Licencia

Proyecto personal de [pacristi](https://github.com/pacristi). Si te sirve, mándame un PR con tu banco y lo agregamos a la lista. 🙂
