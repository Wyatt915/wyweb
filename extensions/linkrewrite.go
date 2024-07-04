package extensions

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var urlRegex = regexp.MustCompile(`^((http|ftp|https)://)?([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:/~+#-]*[\w@?^=%&/~+#-])?`)

func rewriteURL(url []byte, subdir string) ([]byte, error) {
	_, err := os.Stat(string(url))
	if err == nil {
		return url, nil
	}
	path := filepath.Join(subdir, string(url))
	_, err = os.Stat(path)
	if err == nil {
		return []byte(filepath.Join("/", path)), nil
	}
	if urlRegex.Match(url) {
		return url, nil
	}
	return url, errors.New("unknown URL Destination")
}

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
			var err error
			*url, err = rewriteURL(*url, r.subdir)
			if err != nil {
				log.Printf("Error transforming URL '%s' : %s\n", string(*url), err.Error())
			}
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
			util.Prioritized(linkRewriteTransformer{e.subdir}, priorityLinkRewriteTransformer),
		),
	)
}

func LinkRewrite(subdir string) goldmark.Extender {
	return &linkRewrite{subdir}
}
