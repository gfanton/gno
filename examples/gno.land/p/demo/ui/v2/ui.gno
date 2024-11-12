package ui

import (
	"strings"
)

type Element interface {
	String() string
}

func Join(els ...Element) Element {
	var str strings.Builder

	for _, cp := range els {
		str.WriteString(cp.String())
		str.WriteRune('\n')
	}

	return Text(str.String())
}
