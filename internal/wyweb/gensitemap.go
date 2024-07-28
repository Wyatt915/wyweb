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
	sitemapXML.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
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
