package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func rewriteURL(url []byte) ([]byte, error) {
	re := regexp.MustCompile(`^((http|ftp|https)://)?([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:/~+#-]*[\w@?^=%&/~+#-])?`)
	_, err := os.Stat(string(url))
	if err == nil {
		return []byte("LocalLink"), nil
	}
	if re.Match(url) {
		return url, nil
	}
	return url, errors.New("unknown URL Destination")
}

type linkRewriteTransformer struct{}

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
			*url, err = rewriteURL(*url)
			if err != nil {
				fmt.Fprintln(os.Stderr, string(*url), err.Error())
			}
		}
		return ast.WalkContinue, nil
	})
}

type linkRewriteExtension struct{}

func (e *linkRewriteExtension) Extend(m goldmark.Markdown) {
	p := 0
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(linkRewriteTransformer{}, p),
		),
	)
}

func LinkRewriteExtensionExtension() goldmark.Extender {
	return &linkRewriteExtension{}
}
