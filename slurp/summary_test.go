package slurp

import (
	//	"fmt"
	"reflect"
	"testing"
)

func TestCookSummary(t *testing.T) {

	inp := RawSummary{
		"superschoolnews": {"2001-01-02": 19},
		"dailyblah":       {"2001-01-01": 42, "2001-01-03": 102},
	}

	expect := &CookedSummary{
		PubCodes: []string{"dailyblah", "superschoolnews"},
		Days:     []string{"2001-01-01", "2001-01-02", "2001-01-03", "2001-01-04"},
		Data: [][]int{
			[]int{42, 0, 102, 0}, []int{0, 19, 0, 0},
		},
	}

	got := CookSummary(inp, "2001-01-01", "2001-01-04")

	if !reflect.DeepEqual(got, expect) {
		t.Errorf(`CookSummary() failed (got %v", expected %v)`, got, expect)
	}
}
