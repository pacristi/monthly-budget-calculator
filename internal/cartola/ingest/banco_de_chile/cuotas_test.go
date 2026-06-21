package banco_de_chile

import "testing"

func TestParseCuotas(t *testing.T) {
	casos := []struct {
		installments string
		actual       int
		total        int
	}{
		{"01/12", 1, 12},
		{"02/03", 2, 3},
		{"01/01", 1, 1},
		{"1/3", 1, 3},
		{"", 1, 1},
		{"garbage", 1, 1},
		{"1/0", 1, 1},
		{"abc/3", 1, 1},
	}
	for _, c := range casos {
		actual, total := parseCuotas(c.installments)
		if actual != c.actual || total != c.total {
			t.Errorf("parseCuotas(%q) = (%d,%d), quiero (%d,%d)",
				c.installments, actual, total, c.actual, c.total)
		}
	}
}
