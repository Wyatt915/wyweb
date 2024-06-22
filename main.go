package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	wwExt "wyweb.site/wyweb/extensions"
	. "wyweb.site/wyweb/html"
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
			wwExt.EmbedMedia(),
			wwExt.LinkRewrite(subdir),
			//mathjax.MathJax,
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("rainbow_dash"),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
					//chromahtml.WithClasses(true),
					//chromahtml.ClassPrefix("ch"),
					//		chromahtml.LineNumbersInTable(true),
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

func buildDocument(bodyHTML *HTMLElement, headData wmd.HTMLHeadData) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n")
	document := NewHTMLElement("html")
	document.Append(buildHead(headData))
	document.Append(bodyHTML)
	RenderHTML(document, &buf)
	return buf, nil
}

func buildListing(node *wmd.ConfigNode) error {
	meta := (*node.Data).(*wmd.WyWebListing)
	page := NewHTMLElement("article")
	page.AppendNew("nav", Class("navlinks"))
	page.AppendNew("header", Class("listingheader")).AppendNew("h1").AppendText(meta.Title)
	page.AppendNew("div", Class("description")).AppendText(meta.Description)
	children := make([]wmd.ConfigNode, 0)
	for _, child := range node.Children {
		children = append(children, *child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Date.After(children[j].Date)
	})
	for _, child := range children {
		switch post := (*child.Data).(type) {
		case *wmd.WyWebPost:
			listing := page.AppendNew("div", Class("listing"))
			link := listing.AppendNew("a", Href(post.Path))
			link.AppendNew("h2").AppendText(post.Title)
			listing.AppendNew("div", Class("preview")).AppendText(post.Preview)
			tagcontainer := listing.AppendNew("div", Class("tagcontainer"))
			tagcontainer.AppendText("Tags")
			taglist := tagcontainer.AppendNew("div", Class("taglist"))
			for _, tag := range post.Tags {
				taglist.AppendNew("a", Class("taglink"), Href("/tags?tags="+tag)).AppendText(tag)
			}
		default:
			continue
		}
	}
	node.Resolved.HTML = page
	return nil
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

func buildPost(node *wmd.ConfigNode) {
	meta := (*node.Data).(*wmd.WyWebPost)
	resolved := node.Resolved
	var mdtext []byte
	var err error
	if meta.Index != "" {
		mdtext, err = os.ReadFile(filepath.Join(meta.Path, meta.Index))
	} else {
		mdtext, err = findIndex(meta.Path)
	}
	check(err)
	temp, _ := mdConvert(mdtext, meta.Path)
	article := NewHTMLElement("body").AppendNew("article")
	article.AppendText(temp.String())
	resolved.HTML = article
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

	node, err := globalTree.Search(path)
	if err != nil {
		_, ok := os.Stat(filepath.Join(path, "wyweb"))
		if ok != nil {
			w.WriteHeader(404)
			w.Write([]byte(fileNotFound))
			return
		}
		log.Printf("Bizarro error bruv\n")
		return
	}
	meta := node.Data
	resolved := node.Resolved
	if node.Resolved.HTML == nil {
		switch (*meta).(type) {
		//case *WyWebRoot:
		case *wmd.WyWebListing:
			buildListing(node)
		case *wmd.WyWebPost:
			buildPost(node)
		case *wmd.WyWebGallery:
			gallery(node)
		default:
			fmt.Println("whoopsie")
			return
		}
	}
	buf, _ := buildDocument(node.Resolved.HTML, *globalTree.GetHeadData(meta, resolved))
	w.Write(buf.Bytes())

}

func dumpStyles() {
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	css, err := os.Create("monokai.css")
	check(err)
	defer css.Close()
	style := styles.Get("monokai")
	formatter.WriteCSS(css, style)
	for _, sty := range styles.Names() {
		println(sty)
	}
}

func main() {
	dumpStyles()
	sockfile := "/tmp/wyweb.sock"
	socket, err := net.Listen("unix", sockfile)
	check(err)
	grp, err := user.LookupGroup("www-data")
	check(err)
	gid, _ := strconv.Atoi(grp.Gid)
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
