package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"

	"wyweb.site/wyweb/util"
)

func makeTagContainer(tags []string) *HTMLElement {
	tagcontainer := NewHTMLElement("div", Class("tag-container"))
	tagcontainer.AppendText("Tags")
	taglist := tagcontainer.AppendNew("div", Class("tag-list"))
	for _, tag := range tags {
		taglist.AppendNew("a", Class("tag-link"), Href("/tags?tags="+url.QueryEscape(tag))).AppendText(tag)
	}
	return tagcontainer
}

func postToListItem(post *WyWebPost) *HTMLElement {
	listing := NewHTMLElement("div", Class("listing"))
	link := listing.AppendNew("a", Href(post.Path))
	link.AppendNew("h2").AppendText(post.Title)
	listing.AppendNew("div", Class("preview")).AppendText(post.Preview)
	listing.Append(makeTagContainer(post.Tags))
	return listing
}

func galleryItemToListItem(item GalleryItem) *HTMLElement {
	listing := NewHTMLElement("div", Class("listing"))
	link := listing.AppendNew("a", Href(item.Filename))
	link.AppendNew("h2").AppendText(item.Title)
	gl := listing.AppendNew("div", Class("gallery-listing"))
	gl.AppendNew("div", Class("img-container")).AppendNew("a", Href(item.GalleryPath)).AppendNew(
		"img",
		Class("gallery-img"),
		map[string]string{
			"src": filepath.Join(item.GalleryPath, item.Filename),
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

func buildTagListing(tree *ConfigTree, query url.Values, crumbs *HTMLElement) *HTMLElement {
	taglist, ok := query["tags"]
	if !ok {
		panic("No Tags Specified")
	}
	listingData := make([]Listable, 0)
	for _, tag := range taglist {
		listingData = util.ConcatUnique(listingData, tree.GetItemsByTag(tag))
	}
	sort.Slice(listingData, func(i, j int) bool {
		return listingData[i].GetDate().After(listingData[j].GetDate())
	})
	if crumbs == nil {
		crumbs = breadcrumbs(nil, WWNavLink{Path: "/", Text: "Home"}, WWNavLink{Path: "", Text: "Tags"})
	}
	return buildListing(listingData, crumbs, "Tags", fmt.Sprintf("Items tagged with %v", taglist))
}

func buildDirListing(node *ConfigNode) error {
	children := make([]Listable, 0)
	for _, child := range node.Children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].GetDate().After(children[j].GetDate())
	})
	node.Resolved.HTML = buildListing(children, breadcrumbs(node), node.Resolved.Title, node.Resolved.Description)
	return nil
}

func buildListing(items []Listable, breadcrumbs *HTMLElement, title, description string) *HTMLElement {
	body := NewHTMLElement("body")
	page := body.AppendNew("article")
	header := page.AppendNew("header", Class("listing-header"))
	header.Append(breadcrumbs)
	header.AppendNew("h1").AppendText(title)
	page.AppendNew("div", Class("description")).AppendText(description)
	for _, item := range items {
		switch t := item.(type) {
		case *ConfigNode:
			if (*t.Data).GetType() == "post" {
				page.Append(postToListItem((*t.Data).(*WyWebPost)))
			}
		case GalleryItem:
			page.Append(galleryItemToListItem(t))
		default:
			continue
		}
	}
	return body
}
