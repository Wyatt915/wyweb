package main

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
