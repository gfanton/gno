package ui

import (
	"strconv"
	"strings"
)

type ListItemStyle func(idx int, el Element) string

func BulletItemStyle(idx int, el Element) string  { return "* " + el.String() }
func TildItemStyle(idx int, el Element) string    { return "- " + el.String() }
func NumericItemStyle(idx int, el Element) string { return strconv.Itoa(idx+1) + ". " + el.String() }

var defaultItemStyle = BulletItemStyle

type ListItem struct {
	Style   ListItemStyle
	Content Element
}

func NewItem(style ListItemStyle, el Element) *ListItem {
	return &ListItem{style, el}
}

func (item *ListItem) String() string {
	return item.render(1, 1)
}

func (item *ListItem) Copy() *ListItem {
	return &ListItem{Style: item.Style, Content: item.Content}
}

func (item *ListItem) render(idx, depth int) string {
	if depth < 1 {
		depth = 1
	}

	return strings.Repeat("\t", depth-1) + item.Style(idx, item.Content) + "\n"
}

type List struct {
	Items []tNode
}

func (l *List) String() string {
	return l.render(0, 0)
}

func (l *List) render(_, depth int) string {
	return renderNodesTree(depth, l.Items)
}

func NewList(els ...Element) *List {
	return NewListWithStyle(defaultItemStyle, els...)
}

func NewListWithStyle(styl ListItemStyle, els ...Element) *List {
	items := make([]tNode, len(els))

	for i, el := range els {
		switch el := el.(type) {
		case *ListItem:
			// copy item
			item := el.Copy()
			if item.Style == nil {
				item.Style = defaultItemStyle
			}
			items[i] = item
		case *List:
			items[i] = el
		default:
			items[i] = NewItem(styl, el)
		}
	}

	return &List{items}
}

type tNode interface {
	render(idx, depth int) string
}

func renderNodesTree(depth int, nodes []tNode) string {
	var str string

	for i, n := range nodes {
		str += n.render(i, depth+1)
	}

	return str
}
