CREATE TABLE movimientos (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    banco        TEXT    NOT NULL,
    source       TEXT    NOT NULL,
    fecha        TEXT    NOT NULL,
    monto        REAL    NOT NULL,
    descripcion  TEXT    NOT NULL,
    is_usd       INTEGER NOT NULL DEFAULT 0,
    cuotas       TEXT    NOT NULL DEFAULT '',
    raw          TEXT    NOT NULL DEFAULT '{}',
    origen       TEXT    NOT NULL,
    fecha_carga  TEXT    NOT NULL
);

CREATE INDEX idx_mov_dedup ON movimientos (banco, source, fecha, monto, descripcion);
CREATE INDEX idx_mov_fecha ON movimientos (fecha);
