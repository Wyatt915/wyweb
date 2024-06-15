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
	"time"

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

var globalTree *wmd.ConfigTree

const fileNotFound = `
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
</body>
</html>
`

func check(e error) {
	if e != nil {
		fmt.Println(e.Error())
		panic(e)
	}
}
func timer(name string) func() {
	start := time.Now()
	return func() {
		log.Printf("%s took %v\n", name, time.Since(start))
	}
}
func mdConvert(text []byte, subdir string) (bytes.Buffer, error) {
	defer timer("mdConvert")()
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
	buf.WriteString("<body><article><main>")
	buf.Write(bodyHTML)
	buf.WriteString("</main></article></body>\n</html>\n")
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

type WyWebHandler struct {
	http.Handler
}

func (r WyWebHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer timer("ServeHTTP")()
	docRoot := req.Header["Document-Root"][0]
	os.Chdir(docRoot)
	if globalTree == nil {
		var e error
		globalTree, e = wmd.BuildConfigTree(".", "DOMAIN")
		check(e)
	}
	raw := strings.TrimPrefix(req.Header["Request-Uri"][0], "/")
	path, _ := filepath.Rel(".", raw) // remove that pesky leading slash

	var renderedHTML []byte
	meta, resolved, err := globalTree.Search(path)
	if err != nil {
		_, ok := os.Stat(filepath.Join(path, "wyweb"))
		if ok != nil {
			w.WriteHeader(404)
			w.Write([]byte(fileNotFound))
			return
		}
	}
	switch t := (*meta).(type) {
	//case *WyWebRoot:
	case *wmd.WyWebListing:
		renderedHTML, _ = directoryListing(path)
	case *wmd.WyWebPost:
		if resolved.HTML == nil {
			mdtext, err := os.ReadFile(filepath.Join(path, t.Index))
			check(err)
			temp, _ := mdConvert(mdtext, path)
			renderedHTML = temp.Bytes()
			resolved.HTML = &renderedHTML
		} else {
			renderedHTML = *resolved.HTML
		}
	//case *WyWebGallery:
	default:
		fmt.Println("whoopsie")
		return
	}
	buf, _ := buildDocument(renderedHTML, *globalTree.GetHeadData(meta, resolved))
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
	handler := WyWebHandler{}
	//	handler.tree = new(wmd.ConfigTree)
	http.Serve(socket, handler)
}
