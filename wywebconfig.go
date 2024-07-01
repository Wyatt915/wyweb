package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type WWNavLink struct {
	Path string `yaml:"path,omitempty"`
	Text string `yaml:"text,omitempty"`
}

type PageData struct {
	Author      string    `yaml:"author,omitempty"`
	Title       string    `yaml:"title,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Copyright   string    `yaml:"copyright,omitempty"`
	Date        time.Time `yaml:"date,omitempty"`
	Updated     time.Time `yaml:"updated,omitempty"`
	Path        string    `yaml:"path,omitempty"`
	ParentPath  string    `yaml:"parent_path,omitempty"`
	Next        WWNavLink `yaml:"next,omitempty"`
	Prev        WWNavLink `yaml:"prev,omitempty"`
	Up          WWNavLink `yaml:"up,omitempty"`
}

type Resource struct {
	Attributes map[string]string `yaml:"attributes,omitempty"`
	Type       string            `yaml:"type,omitempty"`
	Method     string            `yaml:"method,omitempty"`
	Value      string            `yaml:"value,omitempty"`
	DependsOn  []string          `yaml:"depends_on,omitempty"`
}

type HeadData struct {
	Meta      []string            `yaml:"meta,omitempty"`
	Resources map[string]Resource `yaml:"resources,omitempty"`
	Include   []string            `yaml:"include,omitempty"`
	Exclude   []string            `yaml:"exclude,omitempty"`
}

type WyWebMeta interface {
	GetType() string
	GetPath() string
	GetHeadData() *HeadData
	GetPageData() *PageData
}

type WyWebRoot struct {
	DomainName string `yaml:"domain_name,omitempty"`
	Default    struct {
		Author    string   `yaml:"author,omitempty"`
		Copyright string   `yaml:"copyright,omitempty"`
		Meta      []string `yaml:"meta,omitempty"`
		Resources []string `yaml:"resources,omitempty"`
	} `yaml:"default,omitempty"`
	Always struct {
		Author    string   `yaml:"author,omitempty"`
		Copyright string   `yaml:"copyright,omitempty"`
		Meta      []string `yaml:"meta,omitempty"`
		Resources []string `yaml:"resources,omitempty"`
	} `yaml:"always,omitempty"`
	Index    string `yaml:"index,omitempty"`
	HeadData `yaml:",inline"`
	PageData `yaml:",inline"`
}

type WyWebListing struct {
	HeadData `yaml:",inline"`
	PageData `yaml:",inline"`
}

type WyWebPost struct {
	HeadData `yaml:",inline"`
	PageData `yaml:",inline"`
	Index    string   `yaml:"index,omitempty"`
	Preview  string   `yaml:"preview,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
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
	GalleryPath string
}

func (n GalleryItem) GetDate() time.Time {
	return n.Date
}

type WyWebGallery struct {
	HeadData     `yaml:",inline"`
	PageData     `yaml:",inline"`
	GalleryItems []GalleryItem `yaml:"galleryitems,omitempty"`
}

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
			log.Printf("%+v\n", err)
			return err
		}
		d.Data = &gallery
	default:
		return fmt.Errorf("unknown tag: %s", node.Tag)
	}
	return nil
}

func ReadWyWeb(dir string) (WyWebMeta, error) {
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
