package diagnostics

import "fmt"

type Result struct {
	Name string
	OK   bool
	Fix  string
}

func (r Result) Line() string {
	if r.OK {
		return fmt.Sprintf("%s: OK", r.Name)
	}
	return fmt.Sprintf("%s: FAIL - %s", r.Name, r.Fix)
}
