package main

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"slices"
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
	wmd "wyweb.site/wyweb/metadata"
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

func buildHead(headData wmd.HTMLHeadData) ([]byte, error) {
	links := make([]link, 0)
	for _, style := range headData.Styles {
		switch s := style.(type) {
		case wmd.URLResource:
			links = append(links, link{Rel: "stylesheet", Href: s.String})
			//case RawResource:
		default:
			continue
		}
	}
	data := head{
		Title: headData.Title,
		Link:  links,
	}
	meta := []byte(strings.Join(headData.Meta, "\n"))
	other, err := xml.Marshal(data)
	return slices.Concat(meta, other), err
}

func buildDocument(bodyHTML []byte, head wmd.HTMLHeadData) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n<html>")
	headXML, _ := buildHead(head)
	buf.Write(headXML)
	buf.WriteString("<body>")
	buf.Write(bodyHTML)
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
		meta, e := wmd.ReadWyWeb(path)
		if e != nil {
			fmt.Fprintf(os.Stderr, "%v\n", e)
			return nil
		}
		buf.WriteString(`<a href="`)
		buf.WriteString(path)
		buf.WriteString(fmt.Sprintf(`">%s</a><br />`, meta.GetPageData().Title))
		return nil
	})
	return buf.Bytes(), nil
}

type MyHandler struct {
	http.Handler
	tree *wmd.ConfigTree
}

func (r MyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	docRoot := req.Header["Document-Root"][0]
	os.Chdir(docRoot)
	if r.tree == nil {
		var e error
		r.tree, e = wmd.BuildConfigTree(".", "DOMAIN")
		check(e)
	}
	raw := strings.TrimPrefix(req.Header["Request-Uri"][0], "/")
	path, _ := filepath.Rel(".", raw) // remove that pesky leading slash
	_, err := os.Stat(filepath.Join(path, "wyweb"))
	isWyWeb := err == nil

	var source []byte
	if !isWyWeb {
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
	meta, resolved, err := r.tree.Search(path)
	check(err)
	switch t := (*meta).(type) {
	//case *WyWebRoot:
	case *wmd.WyWebListing:
		source, _ = directoryListing(path)
	case *wmd.WyWebPost:
		mdtext, err := os.ReadFile(filepath.Join(path, t.Index))
		check(err)
		temp, _ := mdConvert(mdtext, path)
		source = temp.Bytes()
	//case *WyWebGallery:
	default:
		fmt.Println("whoopsie")
	}
	buf, _ := buildDocument(source, *r.tree.GetHeadData(meta, resolved))
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
		log.Printf("Failed to change ownership: %v\n", err)
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
