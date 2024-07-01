package main

import (
	"bytes"
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
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	gmText "github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"

	wwExt "wyweb.site/wyweb/extensions"
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
func mdConvert(text []byte, node ConfigNode) (bytes.Buffer, *HTMLElement, *HTMLElement, error) {
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
	if !slices.Contains(node.Resolved.Resources, StyleName) {
		node.Resolved.Resources = append(node.Resolved.Resources, StyleName)
	}
	if !slices.Contains(node.Resolved.Resources, "algol") {
		node.Resolved.Resources = append(node.Resolved.Resources, "algol")
	}

	return buf, renderedToc, title, err
}
func buildArticleHeader(node *ConfigNode, title, article *HTMLElement) {
	header := article.AppendNew("header")
	header.Append(breadcrumbs(node))
	header.Append(title)
	info := header.AppendNew("div", Class("post-info"))
	info.AppendNew("time",
		ID("publication-date"),
		map[string]string{"datetime": node.Date.Format(time.DateOnly)},
	).AppendText(node.Date.Format("Jan _2, 2006"))
	info.AppendNew("span", ID("author")).AppendText(node.Resolved.Author)
	info.AppendNew("time",
		ID("updated"),
		map[string]string{"datetime": node.Updated.Format(time.RFC3339)},
	).AppendText(node.Updated.Format("Jan _2, 2006"))
	navlinks := header.AppendNew("nav", Class("navlinks"))
	navlinks.AppendNew("div",
		ID("navlink-prev"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Resolved.Prev.Path),
	).AppendText(node.Resolved.Prev.Text)
	navlinks.AppendNew("div",
		ID("navlink-up"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Resolved.Up.Path),
	).AppendText(node.Resolved.Up.Text)
	navlinks.AppendNew("div",
		ID("navlink-next"),
		Class("navlink"),
	).AppendNew("a",
		Href(node.Resolved.Next.Path),
	).AppendText(node.Resolved.Next.Text)
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

func buildPost(node *ConfigNode) error {
	meta := (*node.Data).(*WyWebPost)
	resolved := node.Resolved
	var mdtext []byte
	var err error
	if meta.Index != "" {
		mdtext, err = os.ReadFile(filepath.Join(meta.Path, meta.Index))
	} else {
		mdtext, err = findIndex(meta.Path)
	}
	if err != nil {
		return err
	}
	temp, TOC, title, _ := mdConvert(mdtext, *node)
	body := NewHTMLElement("body")
	body.Append(TOC)
	article := body.AppendNew("article")
	buildArticleHeader(node, title, article)
	article.AppendText(temp.String()).NoIndent()
	tagcontainer := article.AppendNew("div", Class("tagcontainer"))
	tagcontainer.AppendText("Tags")
	taglist := tagcontainer.AppendNew("div", Class("taglist"))
	for tag := range node.registeredTags {
		taglist.AppendNew("a", Class("taglink"), Href("/tags?tags="+tag)).AppendText(tag)
	}
	resolved.HTML = body
	return nil
}
