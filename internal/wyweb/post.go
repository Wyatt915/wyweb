package wyweb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	gmText "github.com/yuin/goldmark/text"
	gmUtil "github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/toc"

	wwExt "wyweb.site/extensions"
	"wyweb.site/util"
)

func MagicPost(parent *ConfigNode, name string) *ConfigNode {
	out := &ConfigNode{
		Parent:   parent,
		NodeKind: WWPOST,
		Children: make(map[string]*ConfigNode),
		TagDB:    make(map[string][]Listable),
		Tree:     parent.Tree,
		Index:    filepath.Join(parent.RealPath, name),
		RealPath: filepath.Join(parent.RealPath, name),
	}
	out.Path = filepath.Join(util.TrimMagicSuffix(parent.Path), strings.TrimSuffix(name, ".post.md"))
	meta, err := ReadWyWeb(filepath.Join(parent.RealPath, name), "!post")
	if err == nil {
		meta.(*WyWebPost).Path = filepath.Join(util.TrimMagicSuffix(parent.Path), strings.TrimSuffix(name, ".post.md"))
		out.Data = &meta
	}

	return out
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

func newMarkdown(path string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			wwExt.EmbedMedia(),
			wwExt.AttributeList(),
			wwExt.LinkRewrite(path),
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
}

func ParsePost(md goldmark.Markdown, text []byte, path string) ast.Node {
	reader := gmText.NewReader(text)
	doc := md.Parser().Parse(reader)
	return doc
}

func GetTitleFromMarkdown(node *ConfigNode, text []byte, doc ast.Node) {
	if text == nil {
		text, _ = os.ReadFile(node.Index)
	}
	if doc == nil {
		if node.ParsedDocument != nil {
			doc = *node.ParsedDocument
		} else {
			doc = ParsePost(newMarkdown(node.Path), text, node.Index)
			node.ParsedDocument = &doc
		}
	}
	titleNode := doc.FirstChild()
	for titleNode != nil {
		if titleNode.Kind() == ast.KindHeading && titleNode.(*ast.Heading).Level == 1 {
			break
		}
		titleNode = titleNode.NextSibling()
	}
	if titleNode != nil {
		h1Node := titleNode.(*ast.Heading)
		txt := string(h1Node.Text(text))
		(*node).Title = txt
	}
}

func GetPreviewFromMarkdown(node *ConfigNode, text []byte, doc ast.Node) {
	md := newMarkdown(node.Path)
	if doc == nil {
		if node.ParsedDocument != nil {
			doc = *node.ParsedDocument
		} else {
			doc = ParsePost(md, text, node.Index)
			node.ParsedDocument = &doc
		}
	}
	md.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			gmUtil.Prioritized(&wwExt.PreviewRenderer{}, 0),
		),
	)
	var buf bytes.Buffer
	err := md.Renderer().Render(&buf, text, doc)
	if err == nil {
		node.Preview = buf.String()
	}
}

func MDConvertPost(text []byte, node *ConfigNode) (bytes.Buffer, *HTMLElement, *HTMLElement, error) {
	//node.Lock()
	//defer node.Unlock()
	defer util.Timer("mdConvert")()
	StyleName := "catppuccin-mocha"
	md := newMarkdown(node.Path)
	var doc ast.Node
	if node.ParsedDocument != nil {
		doc = *node.ParsedDocument
	} else {
		doc = ParsePost(md, text, node.Index)
		node.ParsedDocument = &doc
	}
	renderedToc := renderTOC(&doc, text)

	titleNode := doc.FirstChild()
	for titleNode != nil {
		if titleNode.Kind() == ast.KindHeading && titleNode.(*ast.Heading).Level == 1 {
			break
		}
		titleNode = titleNode.NextSibling()
	}
	title := NewHTMLElement("h1", ID("title"))
	if titleNode != nil {
		h1Node := titleNode.(*ast.Heading)
		txt := string(h1Node.Text(text))
		title.AppendText(txt)
		doc.RemoveChild(doc, titleNode)
		if node.Title == "" {
			(*node).Title = txt
		}
	} else {
		title.AppendText(node.Title)
	}

	var buf bytes.Buffer
	var err error
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

func BuildPost(node *ConfigNode) error {
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
	resolved := node
	var mdtext []byte
	var err error
	if node.Index != "" {
		mdtext, err = os.ReadFile(node.Index)
	}
	if err != nil {
		log.Println(err.Error())
		return err
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
	node.StructuredData = append(node.StructuredData, bcSD)
	node.StructuredData = append(node.StructuredData, string(jsonld))
	return nil
}
