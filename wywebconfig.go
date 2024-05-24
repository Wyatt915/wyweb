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

type WyWebRoot struct {
	Author       string `yaml:"author,omitempty"`
	CopyrightMsg string `yaml:"copyright_msg,omitempty"`
	DomainName   string `yaml:"domain_name,omitempty"`
	Location     string `yaml:"location,omitempty"`
	Meta         string `yaml:"meta,omitempty"`
	Available    struct {
		Scripts map[string]WWHeaderInclude `yaml:"scripts,omitempty"`
		Styles  map[string]WWHeaderInclude `yaml:"styles,omitempty"`
	} `yaml:"available,omitempty"`
	Default struct {
		Copyright string   `yaml:"copyright,omitempty"`
		Scripts   []string `yaml:"scripts,omitempty"`
		Styles    []string `yaml:"styles,omitempty"`
	} `yaml:"default,omitempty"`
}

type WyWebPage struct {
	Author       string                     `yaml:"author,omitempty"`
	CopyrightMsg string                     `yaml:"copyright_msg,omitempty"`
	Date         string                     `yaml:"date,omitempty"`
	Location     string                     `yaml:"location,omitempty"`
	ParentPath   string                     `yaml:"parent_path,omitempty"`
	Scripts      map[string]WWHeaderInclude `yaml:"scripts,omitempty"`
	Styles       map[string]WWHeaderInclude `yaml:"styles,omitempty"`
	Title        string                     `yaml:"title,omitempty"`
	Include      struct {
		Scripts []string `yaml:"scripts,omitempty"`
		Styles  []string `yaml:"styles,omitempty"`
	} `yaml:"include,omitempty"`
	Exclude struct {
		Scripts []string `yaml:"scripts,omitempty"`
		Styles  []string `yaml:"styles,omitempty"`
	} `yaml:"exclude,omitempty"`
	NavLinks struct {
		Next WWNavLink `yaml:"next,omitempty"`
		Prev WWNavLink `yaml:"prev,omitempty"`
		Up   WWNavLink `yaml:"up,omitempty"`
	} `yaml:"nav_links,omitempty"`
}

// TODO: Use type composition with the yaml inline tag to include WyWebPage in all of the following.

type WyWebListing struct {
	WyWebPage   `yaml:",inline"`
	Description string `yaml:"description,omitempty"`
}

type WyWebPost struct {
	Index     string    `yaml:"index,omitempty"`
	Tags      []string  `yaml:"tags,omitempty"`
	Updated   time.Time `yaml:"updated,omitempty"`
	WyWebPage `yaml:",inline"`
}

type WyWebGallery struct {
	WyWebPage    `yaml:",inline"`
	Galleryitems []struct {
		Addenda     string   `yaml:"addenda,omitempty"`
		Alt         string   `yaml:"alt,omitempty"`
		Artist      string   `yaml:"artist,omitempty"`
		Date        string   `yaml:"date,omitempty"`
		Description string   `yaml:"description,omitempty"`
		Filename    string   `yaml:"filename,omitempty"`
		Location    string   `yaml:"location,omitempty"`
		Medium      string   `yaml:"medium,omitempty"`
		Title       string   `yaml:"title,omitempty"`
		Tags        []string `yaml:"tags,omitempty"`
	} `yaml:"galleryitems,omitempty"`
}

type WyWebMeta interface {
	GetType() string
}

func (m WyWebRoot) GetType() string {
	return "root"
}

func (m WyWebListing) GetType() string {
	return "listing"
}

func (m WyWebPost) GetType() string {
	return "post"
}

func (m WyWebGallery) GetType() string {
	return "gallery"
}

func (m WyWebPage) GetType() string {
	return "page"
}

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
