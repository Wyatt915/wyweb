package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

func mdConvert(text []byte) (bytes.Buffer, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			MediaExtension(),
			LinkRewriteExtensionExtension(),
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
			//html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	var buf bytes.Buffer
	var err error

	if err = md.Convert(text, &buf); err != nil {
		panic(err)
	}
	return buf, err
}

func buildHead() (bytes.Buffer, error) {
	return *bytes.NewBufferString("<head></head>"), nil
}

func buildDocument(text []byte) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n</html>")
	head, _ := buildHead()
	buf.Write(head.Bytes())
	buf.WriteString("<body>")
	body, _ := mdConvert(text)
	buf.Write(body.Bytes())
	buf.WriteString("</body>\n</html>\n")
	return buf, nil
}

type MyHandler struct {
	http.Handler
}

func (r MyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("================================================================================")
	os.Chdir(req.Header["Document-Root"][0])
	source, err := os.ReadFile(req.Header["Request-Uri"][0])
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(
			`
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
</body>
</html>
		`))
		return
	}
	buf, _ := buildDocument(source)
	w.Write(buf.Bytes())
	fmt.Println("================================================================================")
}

func main() {
	socket, err := net.Listen("unix", "/tmp/wyweb.sock")
	os.Chmod("/tmp/wyweb.sock", 0777)
	check(err)
	// Cleanup the sockfile.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove("/tmp/wyweb.sock")
		os.Exit(1)
	}()
	http.Serve(socket, MyHandler{})
}
