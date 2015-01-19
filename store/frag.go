package store

// attempt to make it easier to construct parameterised SQL statements...
import (
	"fmt"
	"strings"
)

type fragList []frag

// Add an sql fragement + param(s). Use "?" as param placeholder
func (l *fragList) Add(sqlFrag string, params ...interface{}) {
	*l = append(*l, frag{sqlFrag, params})
}
func (frags *fragList) Render(startIdx int, sep string) (string, []interface{}) {

	idx := startIdx
	params := []interface{}{}
	subStrs := []string{}
	for _, f := range *frags {
		s, p := f.build(idx)
		subStrs = append(subStrs, s)
		params = append(params, f.params...)
		idx += len(p)
	}

	return strings.Join(subStrs, sep), params
}

type frag struct {
	fmt    string
	params []interface{}
}

func (f frag) build(baseIdx int) (string, []interface{}) {
	indices := make([]interface{}, len(f.params))
	for i := 0; i < len(f.params); i++ {
		indices[i] = baseIdx + i
	}
	// TODO: replace Sprintf with proper substitution code!
	txt := strings.Replace(f.fmt, "?", "$%d", -1) // use postgresql $N format
	return fmt.Sprintf(txt, indices...), f.params
}
