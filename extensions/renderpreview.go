package extensions

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type PreviewRenderer struct {
	limit int
}

func (r *PreviewRenderer) renderPreview(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	switch node.(type) {
	case *ast.Paragraph:
		if !entering {
			return ast.WalkStop, nil
		}
	case *ast.Heading:
		return ast.WalkSkipChildren, nil
	default:
		return ast.WalkContinue, nil
	}
	return ast.WalkContinue, nil
}

func (r *PreviewRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindParagraph, r.renderPreview)
	reg.Register(ast.KindHeading, r.renderPreview)
}
