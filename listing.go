package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"

	"wyweb.site/wyweb/util"
)

func postToListItem(post *WyWebPost) *HTMLElement {
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

func galleryItemToListItem(item GalleryItem) *HTMLElement {
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
	page := NewHTMLElement("article")
	header := page.AppendNew("header", Class("listingheader"))
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
	return page
}