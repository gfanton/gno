package services

import (
	"fmt"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	mark "github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Gnoland struct {
	client *gnoclient.Client
}

func (g *Gnoland) Realm(pkgPath string, args string) (*ast.Node, error) {
	content, res, err := g.client.Render(pkgPath, args)
	if err != nil {
		return nil, fmt.Errorf("unable Render %q: %w", pkgPath, err)
	}

	if res.Response.IsErr() {
		return nil, fmt.Errorf("response error: %w ", res.Response.Error)
	}

	r := text.NewReader([]byte(content))
	node := mark.DefaultParser().Parse(r)
	return mark.DefaultParser().Parse(r), nil
}
