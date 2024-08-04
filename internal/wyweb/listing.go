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
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wyweb.site/util"
)

func MagicListing(parent *ConfigNode, name string) *ConfigNode {
	out := &ConfigNode{
		Parent:   parent,
		NodeKind: WWLISTING,
		Children: make(map[string]*ConfigNode),
		TagDB:    make(map[string][]Listable),
		Tree:     parent.Tree,
		RealPath: filepath.Join(parent.RealPath, name),
	}
	out.Path = filepath.Join(util.TrimMagicSuffix(parent.Path), util.TrimMagicSuffix(name))
	out.Title = strings.TrimSuffix(name, ".listing")
	meta, e := ReadWyWeb(filepath.Join(parent.Path, name))
	if e == nil {
		out.Data = &meta
	}
	return out
}

func makeTagContainer(tags []string) *HTMLElement {
	tagcontainer := NewHTMLElement("div", Class("tag-container"))
	tagcontainer.AppendText("Tags")
	taglist := tagcontainer.AppendNew("div", Class("tag-list"))
	for _, tag := range tags {
		taglist.AppendNew("a", Class("tag-link"), Href("?tags="+url.QueryEscape(tag))).AppendText(tag)
	}
	return tagcontainer
}

func postToListItem(post *ConfigNode) *HTMLElement {
	listing := NewHTMLElement("div", Class("listing"), ID(post.GetIDb64()))
	link := listing.AppendNew("a", Href(post.Path))
	if post.Title == "" {
		mdfile, err := os.ReadFile(post.Index)
		if err != nil {
			log.Println(err.Error())
			return nil
		}
		GetTitleFromMarkdown(post, mdfile, nil)
	}
	link.AppendNew("h2").AppendText(post.Title)
	if post.Preview == "" {
		mdfile, err := os.ReadFile(post.Index)
		if err != nil {
			log.Println(err.Error())
			return nil
		}
		GetPreviewFromMarkdown(post, mdfile, nil)
	}
	listing.AppendNew("div", Class("preview")).AppendText(post.Preview)
	listing.Append(makeTagContainer(post.Tags))
	return listing
}

func galleryItemToListItem(item *RichImage) *HTMLElement {
	listing := NewHTMLElement("div", Class("listing"))
	link := listing.AppendNew("a", Href(item.ParentPage.Path+"#"+item.GetIDb64()))
	link.AppendNew("h2").AppendText(item.Title)
	gl := listing.AppendNew("div", Class("gallery-listing"))
	gl.AppendNew("div", Class("img-container")).AppendNew("a", Href(item.ParentPage.Path)).AppendNew(
		"img",
		Class("gallery-img"),
		map[string]string{
			"src": filepath.Join(item.ParentPage.Path, item.Filename),
			"alt": item.Alt,
		})
	infoContainer := gl.AppendNew("div", Class("info-container"))
	infoContainer.AppendNew("span", Class("gallery-info-artist")).AppendText(item.Artist)
	infoContainer.AppendNew("span", Class("gallery-info-medium")).AppendText(item.Medium)
	infoContainer.AppendNew("span", Class("gallery-info-location")).AppendText(item.Location)
	infoContainer.AppendNew("span", Class("gallery-info-description")).AppendText(item.Description)
	listing.Append(makeTagContainer(item.Tags))
	return listing
}

func buildTagCloud(node *ConfigNode, cloud *HTMLElement, crumbs *HTMLElement) *HTMLElement {
	body := NewHTMLElement("body")
	header := body.AppendNew("header", Class("listing-header"))
	header.Append(crumbs)
	header.AppendNew("h1").AppendText("Tag Cloud")
	page := body.AppendNew("article")
	//header.AppendNew("div", Class("description")).AppendText(description)
	page.Append(cloud)
	body.Append(BuildFooter(node))
	return body
}

func BuildTagListing(node *ConfigNode, taglist []string, crumbs *HTMLElement) *HTMLElement {
	if len(taglist) == 0 || (len(taglist) == 1 && taglist[0] == "") {
		cloud := NewHTMLElement("body")
		div := cloud.AppendNew("div", Class("tag-cloud"))
		var recordHigh float32 = 0
		var recordLow float32 = math.MaxFloat32
		for _, items := range node.Tree.TagDB {
			if float32(len(items)) > recordHigh {
				recordHigh = float32(len(items))
			}
			if float32(len(items)) < recordLow {
				recordLow = float32(len(items))
			}
		}
		TagDB := node.Tree.TagDB
		if node != node.Tree.Root {
			TagDB = node.TagDB
		}
		for tag, items := range TagDB {
			qs := url.Values(map[string][]string{"tags": {tag}}).Encode()
			div.AppendNew("span", map[string]string{
				"style": fmt.Sprintf("font-size: %3.2frem", 1+(3*(float32(len(items))-recordLow)/(recordHigh-recordLow))),
			}).AppendNew("a", Href("?"+qs)).AppendText(tag)
		}
		return buildTagCloud(node, cloud, crumbs)
	}
	listingData := make([]Listable, 0)
	var msg bytes.Buffer
	if node == node.Tree.Root {
		for _, tag := range taglist {
			listingData = util.ConcatUnique(listingData, node.Tree.GetItemsByTag(tag))
		}
		msg.WriteString(fmt.Sprintf("Items tagged with %v", taglist))
	} else {
		for _, tag := range taglist {
			listingData = util.ConcatUnique(listingData, node.GetItemsByTag(tag))
		}
		msg.WriteString(fmt.Sprintf("Items in %s tagged with %v", node.Title, taglist))
		msg.WriteString("\n<br>\n")
		qs := url.Values(map[string][]string{"tags": taglist}).Encode()
		alltags := NewHTMLElement("a", Href("/tags?"+qs))
		alltags.AppendText(fmt.Sprintf("All items tagged with %v", taglist))
		RenderHTML(alltags, &msg)
	}
	sort.Slice(listingData, func(i, j int) bool {
		return listingData[i].GetDate().After(listingData[j].GetDate())
	})
	if crumbs == nil {
		crumbs, _ = Breadcrumbs(nil, WWNavLink{Path: "/", Text: "Home"}, WWNavLink{Path: "", Text: "Tags"})
	}
	return BuildListing(listingData, crumbs, "Tags", msg.String())
}

func BuildDirListing(node *ConfigNode) error {
	children := make([]Listable, 0)
	for _, child := range node.Children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].GetDate().After(children[j].GetDate())
	})
	crumbs, bcSD := Breadcrumbs(node)
	node.HTML = BuildListing(children, crumbs, node.Title, node.Description)
	node.StructuredData = append(node.StructuredData, bcSD)
	return nil
}

func BuildListing(items []Listable, breadcrumbs *HTMLElement, title, description string) *HTMLElement {
	body := NewHTMLElement("body")
	header := body.AppendNew("header", Class("listing-header"))
	page := body.AppendNew("article", Class("listing-container"))
	header.Append(breadcrumbs)
	header.AppendNew("h1").AppendText(title)
	header.AppendNew("div", Class("description")).AppendText(description)
	for _, item := range items {
		switch t := item.(type) {
		case *ConfigNode:
			if t.NodeKind == WWPOST {
				page.Append(postToListItem(t))
			}
		case *RichImage:
			page.Append(galleryItemToListItem(t))
		default:
			continue
		}
	}
	return body
}
