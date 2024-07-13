package wyweb

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"wyweb.site/util"
)

//go:embed logo.svg
var logoString string

func BuildHead(headData HTMLHeadData) *HTMLElement {
	head := NewHTMLElement("head")
	title := head.AppendNew("title")
	title.AppendText(headData.Title)
	for _, style := range headData.Styles {
		switch s := style.(type) {
		case URLResource:
			head.AppendNew("link", map[string]string{"rel": "stylesheet", "href": s.String}, s.Attributes)
		case RawResource:
			tag := head.AppendNew("style", s.Attributes)
			tag.AppendText(s.String)
		default:
			continue
		}
	}
	for _, script := range headData.Scripts {
		switch s := script.(type) {
		case URLResource:
			head.AppendNew("script", map[string]string{"src": s.String, "async": ""}, s.Attributes)
		case RawResource:
			tag := head.AppendNew("script", s.Attributes)
			tag.AppendText(s.String)
		default:
			continue
		}
	}
	head.AppendText(strings.Join(headData.Meta, "\n"))
	return head
}

func (node *ConfigNode) BuildDocument() (bytes.Buffer, error) {
	return BuildDocument(node.HTML, *node.GetHTMLHeadData(), node.StructuredData...)
}

func BuildDocument(bodyHTML *HTMLElement, headData HTMLHeadData, structuredData ...string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n")
	document := NewHTMLElement("html")
	head := BuildHead(headData)
	for _, data := range structuredData {
		head.AppendNew("script", map[string]string{"type": "application/ld+json"}).AppendText(data)
	}
	document.Append(head)
	document.Append(bodyHTML)
	RenderHTML(document, &buf)
	return buf, nil
}
func BuildFooter(node *ConfigNode) *HTMLElement {
	footer := NewHTMLElement("footer")
	logoContainer := footer.AppendNew("div", Class("wyweb-logo"))
	logoContainer.AppendNew("span").AppendText("Powered by")
	logoContainer.AppendText(logoString)
	logoContainer.AppendNew("a", Href("https://wyweb.site"))
	copyrightMsg := footer.AppendNew("span", Class("copyright"))
	copyrightMsg.AppendText(
		fmt.Sprintf("Copyright © %d %s", time.Now().Year(), node.Copyright),
	)
	return footer
}

func Breadcrumbs(node *ConfigNode, extraCrumbs ...WWNavLink) (*HTMLElement, string) {
	structuredData := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "BreadcrumbList",
	}
	itemListElement := make([]map[string]interface{}, 0)
	nav := NewHTMLElement("nav", AriaLabel("Breadcrumbs"))
	ol := nav.AppendNew("ol", Class("breadcrumbs"))
	var crumbs []WWNavLink
	if node != nil {
		pathList := util.PathToList(node.Path)
		crumbs = make([]WWNavLink, 1+len(pathList))
		temp := node
		idx := len(pathList)
		for temp != nil && idx >= 0 {
			crumbs[idx] = WWNavLink{
				Path: "/" + temp.Path,
				Text: temp.Title,
			}
			idx--
			temp = temp.Parent
		}
	} else {
		crumbs = make([]WWNavLink, 0)
	}
	if extraCrumbs != nil {
		crumbs = slices.Concat(crumbs, extraCrumbs)
	}
	var crumb *HTMLElement
	idx := 1
	for _, link := range crumbs {
		if link.Path == "" || link.Text == "" {
			continue
		}
		fullURL, _ := url.JoinPath("https://"+node.Tree.Domain, link.Path)
		itemListElement = append(itemListElement, map[string]interface{}{
			"@type":    "ListItem",
			"position": idx,
			"name":     link.Text,
			"item":     fullURL,
		})
		idx++
		crumb = ol.AppendNew("li").AppendNew("a", Href(link.Path))
		crumb.AppendText(link.Text)
	}
	structuredData["itemListElement"] = itemListElement
	crumb.Attributes["aria-current"] = "page"
	jsonld, _ := json.MarshalIndent(structuredData, "", "    ")
	return nav, string(jsonld)
}
