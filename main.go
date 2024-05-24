package main

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
		fmt.Println(e.Error())
		panic(e)
	}
}

func mdConvert(text []byte) (bytes.Buffer, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			EmbedMedia(),
			LinkRewriteExtension(),
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

func directoryListing(dir string) ([]byte, error) {
	var buf bytes.Buffer
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {

		buf.WriteString(`<a href="`)
		buf.WriteString(info.Name())
		buf.WriteString(`">Page</a><br />`)
		return nil
	})
	return buf.Bytes(), nil
}

type MyHandler struct {
	http.Handler
}

func (r MyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("================================================================================")
	docRoot := req.Header["Document-Root"][0]
	os.Chdir(docRoot)
	raw := strings.TrimPrefix(req.Header["Request-Uri"][0], "/")
	fmt.Println(raw)
	object, _ := filepath.Rel(".", raw) // remove that pesky leading slash
	fmt.Println(object)
	file, err := os.Stat(object)
	var source []byte
	if err == nil && file.IsDir() {
		readWyWeb(object)
		source, _ = directoryListing(object)
	} else if err == nil {
		source, err = os.ReadFile(object)
		check(err)
	} else {
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
