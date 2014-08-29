package arc

import (
	"testing"
)

func TestSpreadPath(t *testing.T) {
	testData := []struct{ in, out string }{
		{"12345678", "1/12/123"},
		{"2e90f06712788ea6fefe1e613d651e78.warc", "2/2e/2e9"},
	}

	for _, dat := range testData {
		got := spreadPath(dat.in)

		if got != dat.out {
			t.Errorf(`genPath("%s") failed (got "%s", expected "%s")`, dat.in, got, dat.out)
			return
		}
	}

}
