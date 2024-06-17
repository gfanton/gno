package render

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
)

type Renderer struct {
	html.Renderer
}

func (r *Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	r.Renderer.RegisterFuncs(reg)

	reg.Register(ast.NodeKind, renderer.NodeRendererFunc)
}
