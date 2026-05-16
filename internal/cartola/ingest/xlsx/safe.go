package xlsx

import "github.com/extrame/xls"

// safeRow envuelve sheet.Row(i) protegiendo contra el panic de la librería
// extrame/xls cuando la fila i no existe en el sheet. La lib hace
// `row.wb = w.wb` sin chequear nil antes de retornar (worksheet.go:30),
// así que un map lookup fallido se convierte en nil pointer dereference.
//
// Workaround: recover y retornar nil, que es lo que la API debería hacer
// nativamente.
func safeRow(sheet *xls.WorkSheet, i int) (row *xls.Row) {
	defer func() {
		if r := recover(); r != nil {
			row = nil
		}
	}()
	return sheet.Row(i)
}
