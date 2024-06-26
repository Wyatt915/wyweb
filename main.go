package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	gmText "github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"

	wwExt "wyweb.site/wyweb/extensions"
	. "wyweb.site/wyweb/html"
	wmd "wyweb.site/wyweb/metadata"
	"wyweb.site/wyweb/util"
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

func tocRecurse(table *toc.Item, parent *HTMLElement) {
	for _, item := range table.Items {
		child := parent.AppendNew("li")
		child.AppendNew("a", Href("#"+string(item.ID))).AppendText(string(item.Title))
		if len(item.Items) > 0 {
			ul := child.AppendNew("ul")
			tocRecurse(item, ul)
		}
	}
}

func renderTOC(t *toc.TOC) *HTMLElement {
	if len(t.Items) == 0 {
		return nil
	}
	elem := NewHTMLElement("nav", Class("nav-toc"))
	ul := elem.AppendNew("div", Class("toc")).AppendNew("ul")
	for _, item := range t.Items {
		tocRecurse(item, ul)
	}
	if len(ul.Children) == 0 {
		return nil
	}
	return elem
}

func mdConvert(text []byte, node wmd.ConfigNode) (bytes.Buffer, *HTMLElement, *HTMLElement, error) {
	defer timer("mdConvert")()
	StyleName := "catppuccin-mocha"
	md := goldmark.New(
		goldmark.WithExtensions(
			wwExt.EmbedMedia(),
			wwExt.AttributeList(),
			wwExt.LinkRewrite(node.Path),
			extension.GFM,
			extension.Footnote,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
					chromahtml.WithClasses(true),
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

	doc := md.Parser().Parse(gmText.NewReader(text))
	tree, err := toc.Inspect(doc, text, toc.MinDepth(1), toc.MaxDepth(5), toc.Compact(true))
	var renderedToc *HTMLElement
	if err != nil {
		log.Printf("Error generating table of contents\n")
		log.Printf("%+v\n", err)
	} else {
		renderedToc = renderTOC(tree)
	}

	titleNode := doc.FirstChild()
	for titleNode != nil {
		if titleNode.Kind() == ast.KindHeading && titleNode.(*ast.Heading).Level == 1 {
			break
		}
		titleNode = titleNode.NextSibling()
	}
	var title *HTMLElement
	if titleNode != nil {
		h1Node := titleNode.(*ast.Heading)
		title = NewHTMLElement("h1", ID("title"))
		title.AppendText(string(h1Node.Text(text)))
		doc.RemoveChild(doc, titleNode)
	}

	err = md.Renderer().Render(&buf, text, doc)
	if err != nil {
		panic(err)
	}

	formatter := chromahtml.New(chromahtml.WithClasses(true))
	if _, ok := node.Tree.Resources[StyleName]; !ok {
		var css bytes.Buffer
		style := styles.Get(StyleName)
		formatter.WriteCSS(&css, style)
		node.Tree.Resources[StyleName] = wmd.Resource{
			Type:       "style",
			Method:     "raw",
			Value:      css.String(),
			Attributes: map[string]string{"media": "screen"},
		}
	}
	if _, ok := node.Tree.Resources["algol"]; !ok {
		var css bytes.Buffer
		style := styles.Get("algol")
		formatter.WriteCSS(&css, style)
		node.Tree.Resources["algol"] = wmd.Resource{
			Type:       "style",
			Method:     "raw",
			Value:      css.String(),
			Attributes: map[string]string{"media": "print"},
		}
	}
	if !slices.Contains(node.Resolved.Resources, StyleName) {
		node.Resolved.Resources = append(node.Resolved.Resources, StyleName)
	}
	if !slices.Contains(node.Resolved.Resources, "algol") {
		node.Resolved.Resources = append(node.Resolved.Resources, "algol")
	}

	return buf, renderedToc, title, err
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

func breadcrumbs(node *wmd.ConfigNode) *HTMLElement {
	nav := NewHTMLElement("nav", AriaLabel("Breadcrumbs"))
	ol := nav.AppendNew("ol", Class("breadcrumbs"))
	pathList := util.PathToList(node.Path)
	crumbs := make([]wmd.WWNavLink, 1+len(pathList))
	temp := node
	idx := len(pathList)
	for temp != nil && idx >= 0 {
		crumbs[idx] = wmd.WWNavLink{
			Path: "/" + temp.Path,
			Text: temp.Resolved.PageData.Title,
		}
		idx--
		temp = temp.Parent
	}
	var crumb *HTMLElement
	for _, link := range crumbs {
		crumb = ol.AppendNew("li").AppendNew("a", Href(link.Path))
		crumb.AppendText(link.Text)
	}
	crumb.Attributes["aria-current"] = "page"
	return nav
}

func postToListItem(post *wmd.WyWebPost) *HTMLElement {
	listing := NewHTMLElement("div", Class("listing"))
	link := listing.AppendNew("a", Href(post.Path))
	link.AppendNew("h2").AppendText(post.Title)
	listing.AppendNew("div", Class("preview")).AppendText(post.Preview)
	tagcontainer := listing.AppendNew("div", Class("tagcontainer"))
	tagcontainer.AppendText("Tags")
	taglist := tagcontainer.AppendNew("div", Class("taglist"))
	for _, tag := range post.Tags {
		taglist.AppendNew("a", Class("taglink"), Href("/tags?tags="+tag)).AppendText(tag)
	}
	return listing
}

func galleryItemToListItem(item wmd.GalleryItem) *HTMLElement {
	listing := NewHTMLElement("div", Class("listing"))
	link := listing.AppendNew("a", Href(item.Filename))
	link.AppendNew("h2").AppendText(item.Title)
	gl := listing.AppendNew("div", Class("galleryListing"))
	gl.AppendNew("div", Class("imgContainer")).AppendNew("a", Href(item.GalleryPath)).AppendNew(
		"img",
		Class("galleryImg"),
		map[string]string{
			"src": filepath.Join(item.GalleryPath, item.Filename),
			"alt": item.Alt,
		})
	infoContainer := gl.AppendNew("div", Class("infoContainer"))
	infoContainer.AppendNew("span", Class("galleryInfoArtist")).AppendText(item.Artist)
	infoContainer.AppendNew("span", Class("galleryInfoMedium")).AppendText(item.Medium)
	infoContainer.AppendNew("span", Class("galleryInfoLocation")).AppendText(item.Location)
	infoContainer.AppendNew("span", Class("galleryInfoDescription")).AppendText(item.Description)
	tagcontainer := listing.AppendNew("div", Class("tagcontainer"))
	tagcontainer.AppendText("Tags")
	taglist := tagcontainer.AppendNew("div", Class("taglist"))
	for _, tag := range item.Tags {
		taglist.AppendNew("a", Class("taglink"), Href("/tags?tags="+tag)).AppendText(tag)
	}
	return listing
}

func buildTagListing(query url.Values) *HTMLElement {
	taglist, ok := query["tags"]
	if !ok {
		panic("No Tags Specified")
	}
	listingData := make([]wmd.Listable, 0)
	for _, tag := range taglist {
		println(tag)
		listingData = util.ConcatUnique(listingData, globalTree.GetItemsByTag(tag))
	}
	sort.Slice(listingData, func(i, j int) bool {
		return listingData[i].GetDate().After(listingData[j].GetDate())
	})
	return buildListing(listingData, nil, "Tags", fmt.Sprintf("Items tagged with %v", taglist))
}

func buildListing(items []wmd.Listable, breadcrumbs *HTMLElement, title, description string) *HTMLElement {
	page := NewHTMLElement("article")
	header := page.AppendNew("header", Class("listingheader"))
	header.Append(breadcrumbs)
	header.AppendNew("h1").AppendText(title)
	page.AppendNew("div", Class("description")).AppendText(description)
	for _, item := range items {
		switch t := item.(type) {
		case *wmd.ConfigNode:
			log.Printf("%s\t%+v", (*t.Data).GetType(), *t)
			if (*t.Data).GetType() == "post" {
				page.Append(postToListItem((*t.Data).(*wmd.WyWebPost)))
			}
		case wmd.GalleryItem:
			page.Append(galleryItemToListItem(t))
		default:
			continue
		}
	}
	return page
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

func buildArticleHeader(node *wmd.ConfigNode, title, article *HTMLElement) {
	header := article.AppendNew("header")
	header.Append(breadcrumbs(node))
	header.Append(title)
	info := header.AppendNew("div", Class("post-info"))
	info.AppendNew("time",
		ID("publication-date"),
		map[string]string{"datetime": node.Date.Format(time.DateOnly)},
	).AppendText(node.Date.Format("Jan _2, 2006"))
	info.AppendNew("span", ID("author")).AppendText(node.Resolved.PageData.Author)
	info.AppendNew("time",
		ID("updated"),
		map[string]string{"datetime": (*node.Data).(*wmd.WyWebPost).Updated.Format(time.RFC3339)},
	).AppendText(node.Date.Format("Jan _2, 2006"))
	navlinks := header.AppendNew("nav", Class("navlinks"))
	navlinks.AppendNew("div",
		ID("navlink-prev"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Resolved.PageData.Prev.Path),
	).AppendText(node.Resolved.PageData.Prev.Text)
	navlinks.AppendNew("div",
		ID("navlink-up"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Resolved.PageData.Up.Path),
	).AppendText(node.Resolved.PageData.Up.Text)
	navlinks.AppendNew("div",
		ID("navlink-next"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Resolved.PageData.Next.Path),
	).AppendText(node.Resolved.PageData.Next.Text)
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
	temp, TOC, title, _ := mdConvert(mdtext, *node)
	body := NewHTMLElement("body")
	body.Append(TOC)
	article := body.AppendNew("article")
	buildArticleHeader(node, title, article)
	article.AppendText(temp.String()).NoIndent()
	resolved.HTML = body
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
	if path == "tags" {
		query, err := url.ParseQuery(req.URL.RawQuery)
		if err != nil {
			w.WriteHeader(404)
			w.Write([]byte(fileNotFound))
			return
		}
		page := buildTagListing(query)
		headData := globalTree.GetDefaultHead()
		headData.Title = "Tags"
		buf, _ := buildDocument(page, *headData)
		w.Write(buf.Bytes())
		return
	}

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
		switch t := (*meta).(type) {
		//case *WyWebRoot:
		case *wmd.WyWebListing:
			children := make([]wmd.Listable, 0)
			for _, child := range node.Children {
				children = append(children, child)
			}
			sort.Slice(children, func(i, j int) bool {
				return children[i].GetDate().After(children[j].GetDate())
			})
			node.Resolved.HTML = buildListing(children, breadcrumbs(node), t.Title, t.Description)
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

func main() {
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
