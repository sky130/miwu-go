package spec

import (
	"fmt"
	"strconv"
)

type attrIIDs struct {
	siid  int
	iid   int
	index int
}

const (
	languageService          languageType = 1
	languagePropertyOrAction languageType = 2
	languageValueList        languageType = 3
)

func parseIID(s []string, t languageType) (attrIIDs, error) {
	var n int
	switch t {
	case languageService:
		n = 1
	case languagePropertyOrAction:
		n = 2
	case languageValueList:
		n = 3
	default:
		return attrIIDs{}, fmt.Errorf("iid: unknown language type %d", t)
	}

	if len(s) != n*2 || s[0] != "service" {
		return attrIIDs{}, fmt.Errorf("invalid translation key")
	}
	if n >= 2 && s[2] != "property" && s[2] != "action" {
		return attrIIDs{}, fmt.Errorf("invalid translation member")
	}
	if n == 3 && (s[2] != "property" || s[4] != "valuelist") {
		return attrIIDs{}, fmt.Errorf("invalid translation list")
	}
	var i attrIIDs
	dsts := [...]*int{&i.siid, &i.iid, &i.index}
	for k := 0; k < n; k++ {
		v, err := atoiAt(s, 1+2*k)
		if err != nil {
			return attrIIDs{}, err
		}
		if v < 0 || k < 2 && v == 0 {
			return attrIIDs{}, fmt.Errorf("invalid translation IID or index")
		}
		*dsts[k] = v
	}
	return i, nil
}

func atoiAt(s []string, idx int) (int, error) {
	if idx >= len(s) {
		return 0, fmt.Errorf("iid: index %d out of range (len=%d)", idx, len(s))
	}
	v, err := strconv.Atoi(s[idx])
	if err != nil {
		return 0, fmt.Errorf("iid: parse s[%d]=%q: %w", idx, s[idx], err)
	}
	return v, nil
}
