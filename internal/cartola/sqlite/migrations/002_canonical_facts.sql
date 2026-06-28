ALTER TABLE movimientos ADD COLUMN instrumento TEXT NOT NULL DEFAULT 'cuenta_corriente';
ALTER TABLE movimientos ADD COLUMN moneda TEXT NOT NULL DEFAULT 'CLP';
ALTER TABLE movimientos ADD COLUMN cuotas_totales INTEGER NOT NULL DEFAULT 1;

UPDATE movimientos
SET
    instrumento = CASE
        WHEN lower(source) LIKE '%credito%'
            OR lower(source) LIKE '%credit_card%'
            OR lower(source) LIKE 'tc_%'
        THEN 'tarjeta_credito'
        ELSE 'cuenta_corriente'
    END,
    moneda = CASE WHEN is_usd != 0 THEN 'USD' ELSE 'CLP' END,
    cuotas_totales = CASE
        WHEN cuotas GLOB '[0-9][0-9]/[0-9][0-9]'
        THEN CAST(substr(cuotas, 4, 2) AS INTEGER)
        WHEN cuotas GLOB '[0-9]/[0-9]'
        THEN CAST(substr(cuotas, 3, 1) AS INTEGER)
        WHEN cuotas GLOB '[0-9]/[0-9][0-9]'
        THEN CAST(substr(cuotas, 3, 2) AS INTEGER)
        WHEN cuotas GLOB '[0-9][0-9]/[0-9]'
        THEN CAST(substr(cuotas, 4, 1) AS INTEGER)
        ELSE 1
    END;
