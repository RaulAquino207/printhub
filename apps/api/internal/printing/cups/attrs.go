package cups

import "github.com/OpenPrinting/goipp"

// attrs adds typed lookups to a goipp attribute list. IPP attributes are
// multi-valued; these helpers read the first value or all of them.
type attrs goipp.Attributes

func (a attrs) values(name string) goipp.Values {
	for _, attr := range a {
		if attr.Name == name {
			return attr.Values
		}
	}
	return nil
}

func (a attrs) string(name string) string {
	if v := a.values(name); len(v) > 0 {
		return v[0].V.String()
	}
	return ""
}

func (a attrs) strings(name string) []string {
	var out []string
	for _, v := range a.values(name) {
		out = append(out, v.V.String())
	}
	return out
}

func (a attrs) int(name string) (int, bool) {
	if v := a.values(name); len(v) > 0 {
		if i, ok := v[0].V.(goipp.Integer); ok {
			return int(i), true
		}
	}
	return 0, false
}

func (a attrs) ints(name string) []int {
	var out []int
	for _, v := range a.values(name) {
		if i, ok := v.V.(goipp.Integer); ok {
			out = append(out, int(i))
		}
	}
	return out
}

func (a attrs) bool(name string) bool {
	if v := a.values(name); len(v) > 0 {
		b, ok := v[0].V.(goipp.Boolean)
		return ok && bool(b)
	}
	return false
}

// rangeUpper returns the upper bound of a rangeOfInteger attribute.
func (a attrs) rangeUpper(name string) (int, bool) {
	if v := a.values(name); len(v) > 0 {
		if r, ok := v[0].V.(goipp.Range); ok {
			return r.Upper, true
		}
	}
	return 0, false
}
