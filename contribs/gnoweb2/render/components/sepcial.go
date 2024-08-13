package components

import (
	"bytes"
	"context"
	"io"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

type HTMLBlock struct {
	*ast.HTMLBlock
}

// Kind is the kind of hashtag AST nodes.
var Kind = ast.NewNodeKind("gnoSpecial")

type specialBlockParser struct {
}

func NewSpecialBlockParser() parser.BlockParser {
	return &specialBlockParser{}
}

// Trigger returns a list of characters that triggers Parse method of
// this parser.
// If Trigger returns a nil, Open will be called with any lines.
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

// // Kind reports the kind of hashtag nodes.
// func (*Node) Kind() ast.NodeKind { return Kind }

// // Dump dumps the contents of Node to stdout for debugging.
// func (n *Node) Dump(src []byte, level int) {
// 	html.
// 		ast.DumpHelper(n, src, level, map[string]string{
// 		"Tag": string(n.Tag),
// 	}, nil)
// }

func renderSpecial(_ context.Context, w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, error) {
	renderSpecialComponenent(node, entering).Render(context.Background(), w)
	return ast.WalkContinue, nil
}
