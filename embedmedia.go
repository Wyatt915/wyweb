package main

import (
	"bytes"
	//"errors"
	//"fmt"

	//"github.com/yuin/goldmark"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	//"github.com/yuin/goldmark/util"
)

var contextKeySnippet = parser.NewContextKey()

type embedTransformer struct{
    val int
}

func (r embedTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context){
    //var buf bytes.Buffer
    ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error){
        if entering && n.Kind() == ast.KindImage {
            //var imagenode bytes.Buffer
            img := n.(*ast.Image)
            dotidx := bytes.LastIndexByte(img.Destination, '.')
            if dotidx >= 0 {
                ext := img.Destination[dotidx:]
                fmt.Printf("%s:\t%s\n", string(img.Destination),  string(ext))
            }
        }
        return ast.WalkContinue, nil
    })
}

type embedExtension struct{
    dummy int
}

func (e *embedExtension) Extend(m goldmark.Markdown){
    p := int(^uint(0) >> 1) // Lowest priority
    m.Parser().AddOptions(parser.WithASTTransformers(
        util.Prioritized(embedTransformer{0}, p),
    ))
}

func EmbedExtension() goldmark.Extender {
    return &embedExtension{0}
}
