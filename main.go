package main

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
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

	"wyweb.site/wyweb/extensions"
)

func check(e error) {
	if e != nil {
		fmt.Println(e.Error())
		panic(e)
	}
}

func mdConvert(text []byte, subdir string) (bytes.Buffer, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extensions.EmbedMedia(),
			extensions.LinkRewrite(subdir),
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
	var err error

	if err = md.Convert(text, &buf); err != nil {
		panic(err)
	}
	return buf, err
}

type link struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Href  string `xml:"href,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
	Type  string `xml:"type,attr,omitempty"`
}

type head struct {
	Title string `xml:"title,omitempty"`
	Link  []link `xml:"link,omitempty"`
}

func buildHead() ([]byte, error) {
	data := head{
		Title: "Hello, World!",
		Link: []link{
			{Type: "text/css", Href: "/res/styles/master.css"},
		},
	}
	return xml.Marshal(data)
}

func buildDocument(text []byte, subdir string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n<html>")
	headXML, _ := buildHead()
	buf.Write(headXML)
	buf.WriteString("<body>")
	body, _ := mdConvert(text, subdir)
	buf.Write(body.Bytes())
	buf.WriteString("</body>\n</html>\n")
	return buf, nil
}

func directoryListing(dir string) ([]byte, error) {
	var buf bytes.Buffer
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		fmt.Println(path, info.Name())
		wwFileName := filepath.Join(path, "wyweb")
		_, e := os.Stat(wwFileName)
		if e != nil {
			return nil
		}
		meta, e := readWyWeb(path)
		if e != nil {
			fmt.Fprintf(os.Stderr, "%v\n", e)
			return nil
		}
		buf.WriteString(`<a href="`)
		buf.WriteString(path)
		buf.WriteString(fmt.Sprintf(`">%s</a><br />`, meta.(*WyWebPost).Title))
		return nil
	})
	return buf.Bytes(), nil
}

type MyHandler struct {
	http.Handler
}

func (r MyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	docRoot := req.Header["Document-Root"][0]
	os.Chdir(docRoot)
	raw := strings.TrimPrefix(req.Header["Request-Uri"][0], "/")
	object, _ := filepath.Rel(".", raw) // remove that pesky leading slash
	fmt.Println(object)
	_, err := os.Stat(filepath.Join(object, "wyweb"))
	isWyWeb := err == nil

	var source []byte
	if isWyWeb {
		meta, err := readWyWeb(object)
		check(err)
		switch t := meta.(type) {
		//case *WyWebRoot:
		case *WyWebListing:
			source, _ = directoryListing(object)
		case *WyWebPost:
			mdtext, err := os.ReadFile(filepath.Join(object, t.Index))
			check(err)
			temp, _ := mdConvert(mdtext, object)
			source = temp.Bytes()
		//case *WyWebGallery:
		default:
			fmt.Println("whoopsie")
		}
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
	buf, _ := buildDocument(source, object)
	w.Write(buf.Bytes())
}

func main() {
	sockfile := "/tmp/wyweb.sock"
	socket, err := net.Listen("unix", sockfile)
	check(err)
	grp, err := user.LookupGroup("www-data")
	check(err)
	gid, err := strconv.Atoi(grp.Gid)
	if err = os.Chown(sockfile, -1, gid); err != nil {
		fmt.Printf("Failed to change ownership: %v\n", err)
		return
	}
	os.Chmod("/tmp/wyweb.sock", 0660)
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
