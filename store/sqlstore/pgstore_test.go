package sqlstore

import (
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	"time"
)

func ExampleBuildWhere() {

	filt := &store.Filter{
		PubFrom:  time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
		PubTo:    time.Date(2010, 2, 1, 0, 0, 0, 0, time.UTC),
		PubCodes: []string{"dailynews", "dailyshoes"},
	}
	s, p := buildWhere(filt).Render(1, " AND ")

	fmt.Println(s)
	fmt.Println(p)
	// Output:
	// a.published>=$1 AND a.published<$2 AND p.code IN ($3,$4)
	// [2010-01-01 00:00:00 +0000 UTC 2010-02-01 00:00:00 +0000 UTC dailynews dailyshoes]
}
