package extensions

import (
	"log"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	gmutil "github.com/yuin/goldmark/util"

	"wyweb.site/util"
)

type linkRewriteTransformer struct {
	subdir string
}

func (r linkRewriteTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && (n.Kind() == ast.KindLink || n.Kind() == ast.KindImage) {
			var url *[]byte
			switch n.Kind() {
			case ast.KindLink:
				url = &n.(*ast.Link).Destination
			case ast.KindImage:
				url = &n.(*ast.Image).Destination
			}
			temp, err := util.RewriteURLPath(string(*url), r.subdir)
			if err != nil {
				log.Printf("Error transforming URL '%s' : %s\n", string(*url), err.Error())
			}
			*url = []byte(temp)
		}
		return ast.WalkContinue, nil
	})
}

type linkRewrite struct {
	subdir string
}

func (e *linkRewrite) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			gmutil.Prioritized(linkRewriteTransformer{e.subdir}, priorityLinkRewriteTransformer),
		),
	)
}

func LinkRewrite(subdir string) goldmark.Extender {
	return &linkRewrite{subdir}
}
