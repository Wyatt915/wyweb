///////////////////////////////////////////////////////////////////////////////////////////////////
//                                                                                               //
//                                                                                               //
//         oooooo   oooooo     oooo           oooooo   oooooo     oooo         .o8               //
//          `888.    `888.     .8'             `888.    `888.     .8'         "888               //
//           `888.   .8888.   .8' oooo    ooo   `888.   .8888.   .8' .ooooo.   888oooo.          //
//            `888  .8'`888. .8'   `88.  .8'     `888  .8'`888. .8' d88' `88b  d88' `88b         //
//             `888.8'  `888.8'     `88..8'       `888.8'  `888.8'  888ooo888  888   888         //
//              `888'    `888'       `888'         `888'    `888'   888    .o  888   888         //
//               `8'      `8'         .8'           `8'      `8'    `Y8bod8P'  `Y8bod8P'         //
//                                .o..P'                                                         //
//                                `Y8P'                                                          //
//                                                                                               //
//                                                                                               //
//                              Copyright (C) 2024  Wyatt Sheffield                              //
//                                                                                               //
//                 This program is free software: you can redistribute it and/or                 //
//                 modify it under the terms of the GNU General Public License as                //
//                 published by the Free Software Foundation, either version 3 of                //
//                      the License, or (at your option) any later version.                      //
//                                                                                               //
//                This program is distributed in the hope that it will be useful,                //
//                 but WITHOUT ANY WARRANTY; without even the implied warranty of                //
//                 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the                 //
//                          GNU General Public License for more details.                         //
//                                                                                               //
//                   You should have received a copy of the GNU General Public                   //
//                         License along with this program.  If not, see                         //
//                                <https://www.gnu.org/licenses/>.                               //
//                                                                                               //
//                                                                                               //
///////////////////////////////////////////////////////////////////////////////////////////////////

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

func BuildNavlinks(node *ConfigNode) *HTMLElement {
	navlinks := NewHTMLElement("nav", Class("navlinks"))
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
	return navlinks
}
