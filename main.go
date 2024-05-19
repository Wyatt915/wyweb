package main

import (
	"bufio"
	//"fmt"
	"os"

	//"gopkg.in/yaml.v3"
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
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
            EmbedExtension(),
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
    f, err := os.Create("test.html")
    defer f.Close()

    w := bufio.NewWriter(f)

    w.WriteString("<html><body>")
    w.WriteString(buf.String())
    w.WriteString("</body></html>")
}
