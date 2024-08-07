package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

// An HTMLBlock struct represents an html block of Markdown text.
type HTMLBlock struct {
	ast.BaseBlock

	Node *html.Node

	buffer bytes.Buffer

	// ClosureLine is a line that closes this html block.
	ClosureLine text.Segment
}

// IsRaw implements Node.IsRaw.
func (n *HTMLBlock) IsRaw() bool {
	return true
}

// HasClosure returns true if this html block has a closure line,
// otherwise false.
func (n *HTMLBlock) HasClosure() bool {
	return n.ClosureLine.Start >= 0
}

// Dump implements Node.Dump.
func (n *HTMLBlock) Dump(source []byte, level int) {
	fmt.Printf("%v}\n")
}

// KindHTMLBlock is a NodeKind of the HTMLBlock node.
var KindHTMLBlock = ast.NewNodeKind("GnoExtensionBlock")

// Kind implements Node.Kind.
func (n *HTMLBlock) Kind() ast.NodeKind {
	return KindHTMLBlock
}

type specialBlockParser struct {
}

func NewSpecialBlockParser() parser.BlockParser {
	return &specialBlockParser{}
}

func (*specialBlockParser) Trigger() []byte {
	return []byte{'<'}
}

// Open parses the current line and returns a result of parsing.
//
// Open must not parse beyond the current line.
// If Open has been able to parse the current line, Open must advance a reader
// position by consumed byte length.
//
// If Open has not been able to parse the current line, Open should returns
// (nil, NoChildren). If Open has been able to parse the current line, Open
// should returns a new Block node and returns HasChildren or NoChildren.
func (*specialBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	var htmlNode *HTMLBlock

	r, err := html.Parse(nil)

	var buf bytes.Buffer
	line, segment := reader.PeekLine()
	last := pc.LastOpenedBlock().Node
	if pos := pc.BlockOffset(); pos < 0 || line[pos] != '<' {
		return nil, parser.NoChildren
	}

	z := html.NewTokenizer(bytes.NewReader(line))
	tt := z.Next()
	if tt == html.ErrorToken {
		return nil, parser.NoChildren
	}

	tn, hasAttr := z.TagName()
	if hasAttr {
		z.TagAttr()
	}

	return nil, parser.Continue
}

// Continue parses the current line and returns a result of parsing.
//
// Continue must not parse beyond the current line.
// If Continue has been able to parse the current line, Continue must advance
// a reader position by consumed byte length.
//
// If Continue has not been able to parse the current line, Continue should
// returns Close. If Continue has been able to parse the current line,
// Continue should returns (Continue | NoChildren) or
// (Continue | HasChildren)
func (*specialBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	return parser.Continue
}

// Close will be called when the parser returns Close.
func (*specialBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	return
}

// CanInterruptParagraph returns true if the parser can interrupt paragraphs,
// otherwise false.
func (*specialBlockParser) CanInterruptParagraph() bool { return false }

// CanAcceptIndentedLine returns true if the parser can open new node when
// the given line is being indented more than 3 spaces.
func (*specialBlockParser) CanAcceptIndentedLine() bool { return false }
