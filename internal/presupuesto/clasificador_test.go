package presupuesto

import "testing"

func TestClasificar_ReglaPorPatron_FirstMatchWins(t *testing.T) {
	reglas := []Regla{
		{Patron: "fintual ahorro", Destino: "ahorro"},
		{Patron: "fintual", Destino: "inversion"},
	}

	// La descripción matchea ambas reglas; gana la primera de la lista.
	if got := Clasificar("Traspaso Fintual Ahorro mayo", "", reglas, "gasto"); got != "ahorro" {
		t.Errorf("Clasificar(fintual ahorro) = %q, want ahorro", got)
	}

	// Solo matchea la segunda.
	if got := Clasificar("Traspaso Fintual", "", reglas, "gasto"); got != "inversion" {
		t.Errorf("Clasificar(fintual) = %q, want inversion", got)
	}

	// No matchea ninguna regla → categoría default.
	if got := Clasificar("Sushi delivery", "", reglas, "gasto"); got != "gasto" {
		t.Errorf("Clasificar(sin match) = %q, want gasto (default)", got)
	}
}

func TestClasificar_OverrideGanaYIgnorado(t *testing.T) {
	reglas := []Regla{{Patron: "fintual", Destino: "inversion"}}

	// El override manual gana sobre cualquier regla.
	if got := Clasificar("Traspaso Fintual", "ahorro", reglas, "gasto"); got != "ahorro" {
		t.Errorf("Clasificar(override=ahorro) = %q, want ahorro", got)
	}

	// Una regla puede mandar a Ignorado (reemplaza a las exclusiones de hoy).
	reglasIgnorar := []Regla{{Patron: "pago tarjeta", Destino: Ignorado}}
	if got := Clasificar("Pago Tarjeta de Credito", "", reglasIgnorar, "gasto"); got != Ignorado {
		t.Errorf("Clasificar(regla ignorar) = %q, want %q", got, Ignorado)
	}

	// El override también puede forzar Ignorado.
	if got := Clasificar("Sushi delivery", Ignorado, reglas, "gasto"); got != Ignorado {
		t.Errorf("Clasificar(override=ignorado) = %q, want %q", got, Ignorado)
	}
}
