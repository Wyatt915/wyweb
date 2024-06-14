package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type WWHeaderInclude struct {
	Method string `yaml:"method,omitempty"`
	Value  string `yaml:"value,omitempty"`
}

type WWNavLink struct {
	Path string `yaml:"path,omitempty"`
	Text string `yaml:"text,omitempty"`
}

type PageData struct {
	Author     string    `yaml:"author,omitempty"`
	Title      string    `yaml:"title,omitempty"`
	Copyright  string    `yaml:"copyright,omitempty"`
	Date       time.Time `yaml:"date,omitempty"`
	Path       string    `yaml:"path,omitempty"`
	ParentPath string    `yaml:"parent_path,omitempty"`
	NavLinks   struct {
		Next WWNavLink `yaml:"next,omitempty"`
		Prev WWNavLink `yaml:"prev,omitempty"`
		Up   WWNavLink `yaml:"up,omitempty"`
	} `yaml:"nav_links,omitempty"`
}

type HeadData struct {
	Meta    []string                   `yaml:"meta,omitempty"`
	Scripts map[string]WWHeaderInclude `yaml:"scripts,omitempty"`
	Styles  map[string]WWHeaderInclude `yaml:"styles,omitempty"`
	Include struct {
		Scripts []string `yaml:"scripts,omitempty"`
		Styles  []string `yaml:"styles,omitempty"`
	} `yaml:"include,omitempty"`
	Exclude struct {
		Scripts []string `yaml:"scripts,omitempty"`
		Styles  []string `yaml:"styles,omitempty"`
	} `yaml:"exclude,omitempty"`
}

type WyWebRoot struct {
	DomainName string `yaml:"domain_name,omitempty"`
	Available  struct {
		Scripts map[string]WWHeaderInclude `yaml:"scripts,omitempty"`
		Styles  map[string]WWHeaderInclude `yaml:"styles,omitempty"`
	} `yaml:"available,omitempty"`
	Default struct {
		Author    string   `yaml:"author,omitempty"`
		Copyright string   `yaml:"copyright,omitempty"`
		Meta      []string `yaml:"meta,omitempty"`
		Scripts   []string `yaml:"scripts,omitempty"`
		Styles    []string `yaml:"styles,omitempty"`
	} `yaml:"default,omitempty"`
	Always struct {
		Author    string   `yaml:"author,omitempty"`
		Copyright string   `yaml:"copyright,omitempty"`
		Meta      []string `yaml:"meta,omitempty"`
		Scripts   []string `yaml:"scripts,omitempty"`
		Styles    []string `yaml:"styles,omitempty"`
	} `yaml:"always,omitempty"`
	Index    string `yaml:"index,omitempty"`
	HeadData `yaml:",inline"`
	PageData `yaml:",inline"`
}

// TODO: Use type composition with the yaml inline tag to include WyWebPage in all of the following.

type WyWebListing struct {
	HeadData    `yaml:",inline"`
	PageData    `yaml:",inline"`
	Description string `yaml:"description,omitempty"`
}

type WyWebPost struct {
	HeadData `yaml:",inline"`
	PageData `yaml:",inline"`
	Index    string    `yaml:"index,omitempty"`
	Tags     []string  `yaml:"tags,omitempty"`
	Updated  time.Time `yaml:"updated,omitempty"`
}

type GalleryItem struct {
	Addenda     string    `yaml:"addenda,omitempty"`
	Alt         string    `yaml:"alt,omitempty"`
	Artist      string    `yaml:"artist,omitempty"`
	Date        time.Time `yaml:"date,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Filename    string    `yaml:"filename,omitempty"`
	Location    string    `yaml:"location,omitempty"`
	Medium      string    `yaml:"medium,omitempty"`
	Title       string    `yaml:"title,omitempty"`
	Tags        []string  `yaml:"tags,omitempty"`
}

type WyWebGallery struct {
	HeadData     `yaml:",inline"`
	PageData     `yaml:",inline"`
	GalleryItems []GalleryItem `yaml:"galleryitems,omitempty"`
}

type WyWebMeta interface {
	GetType() string
	GetPath() string
	GetHeadData() *HeadData
	GetPageData() *PageData
}

//func AsPage(meta *WyWebMeta) (*WyWebPage, error) {
//	switch (*meta).(type) {
//	case *WyWebRoot:
//		return nil, fmt.Errorf("cannot represent WyWebRoot as a page type")
//	case *WyWebPage:
//		return (*meta).(*WyWebPage), nil
//	case *WyWebPost, *WyWebListing, *WyWebGallery:
//		out := &WyWebPage{
//			Author:     (*meta).Author,
//			Copyright:  (*meta).Copyright,
//			Date:       (*meta).Date,
//			Path:       (*meta).Path,
//			ParentPath: (*meta).ParentPath,
//			Scripts:    make(map[string]WWHeaderInclude),
//			Styles:     make(map[string]WWHeaderInclude),
//			Title:      (*meta).Title,
//			Include: struct {
//				Styles  []string
//				Scripts []string
//			}{
//				Styles:  make([]string, len(meta.Include.Styles)),
//				Scripts: make([]string, len(meta.Include.Scripts)),
//			},
//			Exclude: struct {
//				Styles  []string
//				Scripts []string
//			}{
//				Styles:  make([]string, len(meta.Exclude.Styles)),
//				Scripts: make([]string, len(meta.Exclude.Scripts)),
//			},
//			NavLinks: (*meta).NavLinks,
//		}
//		return &out, nil
//	default:
//		return nil, fmt.Errorf("unknown error representing wyweb page")
//	}
//}

// //////////////////////////////////////////////////////////////////////////////
//
//	WyWebRoot methods
//
// //////////////////////////////////////////////////////////////////////////////
func (m WyWebRoot) GetPath() string {
	return m.Path
}

func (m WyWebRoot) GetType() string {
	return "root"
}

func (m WyWebRoot) GetHeadData() *HeadData {
	return &m.HeadData
}

func (m WyWebRoot) GetPageData() *PageData {
	return &m.PageData
}

// //////////////////////////////////////////////////////////////////////////////
//
//	WyWebListing methods
//
// //////////////////////////////////////////////////////////////////////////////
func (m WyWebListing) GetPath() string {
	return m.Path
}

func (m WyWebListing) GetType() string {
	return "listing"
}

func (m WyWebListing) GetHeadData() *HeadData {
	return &m.HeadData
}

func (m WyWebListing) GetPageData() *PageData {
	return &m.PageData
}

// //////////////////////////////////////////////////////////////////////////////
//
//	WyWebPost methods
//
// //////////////////////////////////////////////////////////////////////////////
func (m WyWebPost) GetPath() string {
	return m.Path
}

func (m WyWebPost) GetType() string {
	return "post"
}

func (m WyWebPost) GetHeadData() *HeadData {
	return &m.HeadData
}

func (m WyWebPost) GetPageData() *PageData {
	return &m.PageData
}

// //////////////////////////////////////////////////////////////////////////////
//
//	WyWebGallery methods
//
// //////////////////////////////////////////////////////////////////////////////
func (m WyWebGallery) GetPath() string {
	return m.Path
}

func (m WyWebGallery) GetType() string {
	return "gallery"
}

func (m WyWebGallery) GetHeadData() *HeadData {
	return &m.HeadData
}

func (m WyWebGallery) GetPageData() *PageData {
	return &m.PageData
}

// //////////////////////////////////////////////////////////////////////////////
//
//	WyWebPage methods
//
// //////////////////////////////////////////////////////////////////////////////
//func (m WyWebPage) GetPath() string {
//	return m.Path
//}
//
//func (m WyWebPage) GetType() string {
//	return "page"
//}

type Document struct {
	Data WyWebMeta
}

func (d *Document) UnmarshalYAML(node *yaml.Node) error {
	switch strings.ToLower(node.Tag) {
	case "!root":
		var root WyWebRoot
		if err := node.Decode(&root); err != nil {
			return err
		}
		d.Data = &root
	case "!listing":
		var listing WyWebListing
		if err := node.Decode(&listing); err != nil {
			return err
		}
		d.Data = &listing
	case "!post":
		var post WyWebPost
		if err := node.Decode(&post); err != nil {
			return err
		}
		d.Data = &post
	case "!gallery":
		var gallery WyWebGallery
		if err := node.Decode(&gallery); err != nil {
			return err
		}
		d.Data = &gallery
	default:
		return fmt.Errorf("unknown tag: %s", node.Tag)
	}
	return nil
}

func readWyWeb(dir string) (WyWebMeta, error) {
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	filename := filepath.Join(dir, "wyweb")
	wywebData, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var meta Document
	err = yaml.Unmarshal(wywebData, &meta)
	if err != nil {
		return nil, err
	}
	return meta.Data, nil
}
