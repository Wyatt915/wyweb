package main

import (
    "os"
    "fmt"
    //"gopkg.in/yaml.v3"
    "bytes"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/yuin/goldmark/parser"
    "github.com/yuin/goldmark/renderer/html"
    highlighting "github.com/yuin/goldmark-highlighting/v2"
    chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
)

func check(e error) {
    if e != nil {
        panic(e)
    }
}

func main() {
    source, err := os.ReadFile("./sample-doc.md")
    check(err)

    md := goldmark.New(
        goldmark.WithExtensions(
            extension.GFM,
            highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
				),
			),
        ),
        goldmark.WithParserOptions(
            parser.WithAutoHeadingID(),
        ),
        goldmark.WithRendererOptions(
            html.WithXHTML(),
            html.WithUnsafe(),
        ),
    )
    var buf bytes.Buffer
    if err := md.Convert([]byte(source), &buf); err != nil {
        panic(err)
    }
    fmt.Println("<html><head><title>This is a test document</title></head><body>")
    fmt.Println(string(buf.Bytes()))
    fmt.Println("</body></html>")
}
