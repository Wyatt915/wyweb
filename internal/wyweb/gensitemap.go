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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (node *ConfigNode) buildSitemap(urlset *HTMLElement, baseURL string) {
	url := urlset.AppendNew("url")
	url.AppendNew("loc").AppendText(baseURL + node.Path)
	if !node.Updated.IsZero() {
		url.AppendNew("lastmod").AppendText((*node).Updated.Format(time.DateOnly))
	}
	for _, child := range node.Children {
		child.buildSitemap(urlset, baseURL)
	}
	files, err := os.ReadDir(node.Path)
	if err != nil {
		return
	}
	for _, file := range files {
		name := file.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".jpg", ".jpeg", ".gif", ".png", ".svg", ".webp", ".tiff":
			path := filepath.Join(node.Path, name)
			url.AppendNew("image:image").AppendNew("image:loc").AppendText(baseURL + path)
		}
	}
}

func (tree *ConfigTree) MakeSitemap() {
	var sitemapXML bytes.Buffer
	sitemapXML.WriteString(`<?xml version="1.0" encoding="UTF-8" ?>`)
	sitemapXML.WriteByte('\n')
	urlset := NewHTMLElement("urlset", map[string]string{
		"xmlns":       "http://www.sitemaps.org/schemas/sitemap/0.9",
		"xmlns:image": "http://www.google.com/schemas/sitemap-image/1.1",
		"xmlns:video": "http://www.google.com/schemas/sitemap-video/1.1",
	})
	baseURL := "https://" + tree.Domain + "/"
	tree.Root.buildSitemap(urlset, baseURL)
	RenderHTML(urlset, &sitemapXML)
	sitemapFile, err := os.Create("sitemap.xml")
	if err != nil && !os.IsExist(err) {
		fmt.Printf("%+v\n", err)
		return
	}
	defer sitemapFile.Close()
	sitemapFile.Write(sitemapXML.Bytes())
}

func (node *ConfigNode) MakeRSS() []Listable {
	if !(node.NodeKind == WWGALLERY || node.NodeKind == WWLISTING) {
		return nil
	}
	path := filepath.Join(node.RealPath, "rssfeed.xml")
	rssFile, err := os.Create(path)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("%+v\n", err)
		return nil
	}
	defer rssFile.Close()
	var rssXML bytes.Buffer
	rssXML.WriteString(`<?xml version="1.0" encoding="UTF-8" ?>`)
	rssXML.WriteByte('\n')
	baseURL := "https://" + node.Tree.Domain + "/"
	feed := NewHTMLElement("rss", map[string]string{
		"version":    "2.0",
		"xmlns:atom": "http://www.w3.org/2005/Atom",
	})
	channel := feed.AppendNew("channel")
	channel.AppendNew("title").AppendText(node.Title)
	channel.AppendNew("description").AppendText(node.Description)
	channel.AppendNew("link").AppendText(baseURL + node.Path)
	channel.AppendNew("copyright").AppendText(node.Copyright)
	channel.AppendNew("lastBuildDate").AppendText(time.Now().Format(time.RFC1123Z))
	pubDate, _ := node.getMostRecentDates()
	channel.AppendNew("pubDate").AppendText(pubDate.Format(time.RFC1123Z))
	channel.AppendNew("ttl").AppendText("60")
	channel.AppendNew("atom:link", Href(baseURL+path), map[string]string{"rel": "self", "type": "application/rss+xml"}).SetSelfClosing(true)

	children := make([]Listable, 0)
	switch node.NodeKind {
	case WWLISTING:
		for _, child := range node.Children {
			children = append(children, child)
		}
	case WWGALLERY:
		for _, child := range node.Images {
			children = append(children, &child)
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].GetDate().After(children[j].GetDate())
	})
	for _, child := range children {
		channel.Append(child.AsRSSItem())
	}
	RenderHTML(feed, &rssXML)
	rssFile.Write(rssXML.Bytes())
	return children
}

func (tree *ConfigTree) MakeRSS() {
	path := "rssfeed.xml"
	rssFile, err := os.Create(path)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("%+v\n", err)
		return
	}
	defer rssFile.Close()
	var rssXML bytes.Buffer
	rssXML.WriteString(`<?xml version="1.0" encoding="UTF-8" ?>`)
	rssXML.WriteByte('\n')
	baseURL := "https://" + tree.Domain + "/"
	feed := NewHTMLElement("rss", map[string]string{
		"version":    "2.0",
		"xmlns:atom": "http://www.w3.org/2005/Atom",
	})
	channel := feed.AppendNew("channel")
	title := tree.Domain
	if tree.Root.Title != "" {
		title = tree.Root.Title
	}
	channel.AppendNew("title").AppendText(title)
	channel.AppendNew("description").AppendText(tree.Root.Description)
	channel.AppendNew("link").AppendText(baseURL)
	channel.AppendNew("copyright").AppendText(tree.Root.Copyright)
	channel.AppendNew("lastBuildDate").AppendText(time.Now().Format(time.RFC1123Z))
	pubDate, _ := tree.Root.getMostRecentDates()
	channel.AppendNew("pubDate").AppendText(pubDate.Format(time.RFC1123Z))
	channel.AppendNew("ttl").AppendText("60")
	channel.AppendNew("atom:link", Href(baseURL+path), map[string]string{"rel": "self", "type": "application/rss+xml"}).SetSelfClosing(true)

	items := make([]Listable, 0)
	var dft func(*ConfigNode)
	dft = func(node *ConfigNode) {
		temp := node.MakeRSS()
		if temp != nil {
			items = append(items, temp...)
		}
		for _, child := range node.Children {
			dft(child)
		}
	}
	dft(tree.Root)
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetDate().After(items[j].GetDate())
	})
	for _, item := range items {
		channel.Append(item.AsRSSItem())
	}
	RenderHTML(feed, &rssXML)
	rssFile.Write(rssXML.Bytes())
}
