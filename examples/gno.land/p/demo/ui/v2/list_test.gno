package ui

import (
	"fmt"
	"testing"
)

func TestList(t *testing.T) {
	list := NewListWithStyle(NumericItemStyle,
		Text("El1"),
		Text("El2"),
		&ListItem{Content: "NopNop", Style: BulletItemStyle},
		Text("El3"),
		NewListWithStyle(NumericItemStyle,
			Text("El4"),
			Text("El5"),
			NewList(Text("El6")),
			Text("El7"),
		),
		Text("El8"),
	)

	render := list.String()
	fmt.Println(render)
}
