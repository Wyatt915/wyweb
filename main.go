package main

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
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

func buildHead(headData wmd.HTMLHeadData) *HTMLElement {
	head := NewHTMLElement("head")
	title := head.AppendNew("title")
	title.AppendText(headData.Title)
	for _, style := range headData.Styles {
		switch s := style.(type) {
		case wmd.URLResource:
			head.AppendNew("link", map[string]string{"rel": "stylesheet", "href": s.String}, s.Attributes)
		case wmd.RawResource:
			tag := head.AppendNew("style", s.Attributes)
			tag.AppendText(s.String)
		default:
			continue
		}
	}
	for _, script := range headData.Scripts {
		switch s := script.(type) {
		case wmd.URLResource:
			head.AppendNew("script", map[string]string{"src": s.String, "async": ""}, s.Attributes)
		case wmd.RawResource:
			tag := head.AppendNew("script", s.Attributes)
			tag.AppendText(s.String)
		default:
			continue
		}
	}
	head.AppendText(strings.Join(headData.Meta, "\n"))
	return head
}

func buildDocument(bodyHTML []byte, headData wmd.HTMLHeadData) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n")
	document := NewHTMLElement("html")
	head := buildHead(headData)
	document.Append(head)
	document.AppendNew("body").AppendNew("article").AppendNew("main").AppendText(string(bodyHTML))
	RenderHTML(document, &buf)
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

func findIndex(path string) ([]byte, error) {
	tryFiles := []string{
		"article.md",
		"index.md",
		"post.md",
		"article",
		"index",
		"post",
	}
	for _, f := range tryFiles {
		index := filepath.Join(path, f)
		_, err := os.Stat(index)
		if err == nil {
			return os.ReadFile(index)
		}
	}
	return nil, fmt.Errorf("could not find index")
}

func buildPost(meta *wmd.WyWebPost, resolved *wmd.Distillate) {
	var mdtext []byte
	var err error
	if meta.Index != "" {
		mdtext, err = os.ReadFile(filepath.Join(meta.Path, meta.Index))
	} else {
		mdtext, err = findIndex(meta.Path)
	}
	check(err)
	temp, _ := mdConvert(mdtext, meta.Path)
	renderedHTML := temp.Bytes()
	resolved.HTML = &renderedHTML
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
		fmt.Printf("%+v\n\n%+v\n\n%+v\n", meta, resolved, err)
		return
	}
	switch wyweb := (*meta).(type) {
	//case *WyWebRoot:
	case *wmd.WyWebListing:
		renderedHTML, _ = directoryListing(path)
	case *wmd.WyWebPost:
		if resolved.HTML == nil {
			buildPost(wyweb, resolved)
		}
		renderedHTML = *resolved.HTML

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
