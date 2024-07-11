package wyweb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	gmText "github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"

	wwExt "wyweb.site/extensions"
	"wyweb.site/util"
)

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

func renderTOC(doc *ast.Node, text []byte) *HTMLElement {
	tree, err := toc.Inspect(*doc, text, toc.MinDepth(1), toc.MaxDepth(5), toc.Compact(true))
	if err != nil {
		log.Printf("Error generating table of contents\n")
		log.Printf("%+v\n", err)
		return nil
	}
	if len(tree.Items) == 0 {
		return nil
	}
	elem := NewHTMLElement("nav", Class("nav-toc"))
	ul := elem.AppendNew("div", Class("toc")).AppendNew("ul")
	for _, item := range tree.Items {
		tocRecurse(item, ul)
	}
	if len(ul.Children) == 0 {
		return nil
	}
	return elem
}

func MDConvertPost(text []byte, node *ConfigNode) (bytes.Buffer, *HTMLElement, *HTMLElement, error) {
	//node.Lock()
	//defer node.Unlock()
	defer util.Timer("mdConvert")()
	StyleName := "catppuccin-mocha"
	PostMD := goldmark.New(
		goldmark.WithExtensions(
			wwExt.EmbedMedia(),
			wwExt.AttributeList(),
			wwExt.LinkRewrite(node.Path),
			wwExt.AlertExtension(),
			meta.Meta,
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
	reader := gmText.NewReader(text)

	doc := PostMD.Parser().Parse(reader)
	//doc.Dump(reader.Source(), 0)
	renderedToc := renderTOC(&doc, text)

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

	err = PostMD.Renderer().Render(&buf, text, doc)
	if err != nil {
		panic(err)
	}

	formatter := chromahtml.New(chromahtml.WithClasses(true))
	if _, ok := node.Tree.Resources[StyleName]; !ok {
		var css bytes.Buffer
		style := styles.Get(StyleName)
		formatter.WriteCSS(&css, style)
		node.Tree.Resources[StyleName] = Resource{
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
		node.Tree.Resources["algol"] = Resource{
			Type:       "style",
			Method:     "raw",
			Value:      css.String(),
			Attributes: map[string]string{"media": "print"},
		}
	}
	if !slices.Contains(node.LocalResources, StyleName) {
		node.LocalResources = append(node.LocalResources, StyleName)
	}
	if !slices.Contains(node.LocalResources, "algol") {
		node.LocalResources = append(node.LocalResources, "algol")
	}

	return buf, renderedToc, title, err
}
func buildArticleHeader(node *ConfigNode, title, crumbs, article *HTMLElement) {
	//node.RLock()
	//defer node.RUnlock()
	header := article.AppendNew("header")
	header.Append(crumbs)
	header.Append(title)
	info := header.AppendNew("div", Class("post-info"))
	info.AppendNew("time",
		ID("publication-date"),
		map[string]string{"datetime": node.Date.Format(time.DateOnly)},
	).AppendText(node.Date.Format("Jan _2, 2006"))
	info.AppendNew("span", ID("author")).AppendText(node.Author)
	info.AppendNew("time",
		ID("updated"),
		map[string]string{"datetime": node.Updated.Format(time.RFC3339)},
	).AppendText(node.Updated.Format("Jan _2, 2006"))
	navlinks := header.AppendNew("nav", Class("navlinks"))
	navlinks.AppendNew("div",
		ID("navlink-prev"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Prev.Path),
	).AppendText(node.Prev.Text)
	navlinks.AppendNew("div",
		ID("navlink-up"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Up.Path),
	).AppendText(node.Up.Text)
	navlinks.AppendNew("div",
		ID("navlink-next"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Next.Path),
	).AppendText(node.Next.Text)
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

func BuildPost(node *ConfigNode) ([]string, error) {
	//node.RLock()
	//defer node.RUnlock()
	structuredData := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "BlogPosting",
		"author": map[string]interface{}{
			"@type": "person",
			"name":  node.Author,
		},
		"headline":      node.Title,
		"datePublished": node.Date.Format(time.DateOnly),
		"dateUpdated":   node.Updated.Format(time.DateOnly),
	}
	meta := (*node.Data).(*WyWebPost)
	resolved := node
	var mdtext []byte
	var err error
	if meta.Index != "" {
		mdtext, err = os.ReadFile(filepath.Join(meta.Path, meta.Index))
	} else {
		mdtext, err = findIndex(meta.Path)
	}
	if err != nil {
		return nil, err
	}
	temp, TOC, title, _ := MDConvertPost(mdtext, node)
	body := NewHTMLElement("body")
	body.Append(TOC)
	article := body.AppendNew("article")
	crumbs, bcSD := Breadcrumbs(node)
	buildArticleHeader(node, title, crumbs, article)
	article.AppendText(temp.String()).NoIndent()
	tagcontainer := article.AppendNew("div", Class("tag-container"))
	tagcontainer.AppendText("Tags")
	taglist := tagcontainer.AppendNew("div", Class("tag-list"))
	for _, tag := range node.Tags {
		taglist.AppendNew("a", Class("tag-link"), Href("/"+filepath.Join(node.Parent.Path, "?tags=")+tag)).AppendText(tag)
	}
	resolved.HTML = body
	jsonld, _ := json.MarshalIndent(structuredData, "", "    ")
	return []string{string(jsonld), bcSD}, nil
}
