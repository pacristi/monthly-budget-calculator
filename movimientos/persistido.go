package movimientos

// Persistido es un MovimientoBruto que ya tiene un ID asignado por el
// store (sqlite). El ID es necesario porque los overrides se referencian
// por "sql-{id}" — ver movimientos/sqlite/adapter.go, que construye ese
// string con fmt.Sprintf("sql-%d", id).
type Persistido struct {
	ID int64
	MovimientoBruto
}
