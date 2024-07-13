package wyweb

import (
	"fmt"
	"log"
	"math/bits"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type WWNodeKind int

const (
	WWNULL WWNodeKind = iota
	WWROOT
	WWPOST
	WWGALLERY
	WWLISTING
	WWPODCAST
	WWRECIPE
)

var KindNames = map[WWNodeKind]string{
	WWROOT:    "root",
	WWPOST:    "post",
	WWGALLERY: "gallery",
	WWLISTING: "listing",
	WWPODCAST: "podcast",
	WWRECIPE:  "recipe",
}

type WWNavLink struct {
	Path string `yaml:"path,omitempty"`
	Text string `yaml:"text,omitempty"`
}

func (r WWNavLink) IsZero() bool {
	return r.Path == "" && r.Text == ""
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
	GetType() WWNodeKind
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

type RichImage struct {
	id          uint64
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
	ParentPage  *ConfigNode
}

func (n *RichImage) GetDate() time.Time {
	return n.Date
}

func (n *RichImage) GetID() uint64 {
	return n.id
}

func (n *RichImage) GetTitle() string {
	return n.Title
}

func (n *RichImage) SetID() {
	epoch := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	age := uint64((n.Date.Sub(epoch)).Hours()/24) & 0xFFFF // 16 bits will last until June 2179
	n.id = (age << (64 - 16))
	bitsRemaining := 64 - 16
	for _, char := range n.Filename {
		runeSize := 32 - bits.LeadingZeros32(uint32(char))
		//fmt.Printf("%s%b\n", strings.Repeat(" ", 64-bitsRemaining), uint32(char))
		if runeSize < bitsRemaining {
			n.id = n.id | uint64(char)<<(bitsRemaining-runeSize)
		} else {
			n.id = n.id | uint64(char)>>(runeSize-bitsRemaining)
			break
		}
		bitsRemaining -= runeSize
	}
	//fmt.Printf("%064b\n\n", n.id)
}

func (r *RichImage) StructuredData() interface{} {
	out := map[string]interface{}{
		"@context": "https://schema.org/",
		"@type":    "ImageObject",
		"contentURL": url.URL{
			Scheme: "https",
			Host:   r.ParentPage.Tree.Domain,
			Path:   filepath.Join(r.ParentPage.RealPath, r.Filename),
		},
		"creator": map[string]interface{}{
			"@type": "Person",
			"name":  r.Artist,
		},
	}
	return out
}

type WyWebGallery struct {
	HeadData     `yaml:",inline"`
	PageData     `yaml:",inline"`
	GalleryItems []RichImage `yaml:"galleryitems,omitempty"`
}

// //////////////////////////////////////////////////////////////////////////////
//
//	WyWebRoot methods
//
// //////////////////////////////////////////////////////////////////////////////
func (m WyWebRoot) GetPath() string {
	return m.Path
}

func (m WyWebRoot) GetType() WWNodeKind {
	return WWROOT
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

func (m WyWebListing) GetType() WWNodeKind {
	return WWLISTING
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

func (m WyWebPost) GetType() WWNodeKind {
	return WWPOST
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

func (m WyWebGallery) GetType() WWNodeKind {
	return WWGALLERY
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
	Data     WyWebMeta
	forceTag string
}

func (d *Document) UnmarshalYAML(node *yaml.Node) error {
	var tag string = strings.ToLower(node.Tag)
	if d.forceTag != "" {
		tag = d.forceTag
	}
	switch tag {
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
		for idx := range gallery.GalleryItems {
			gallery.GalleryItems[idx].SetID()
		}
		d.Data = &gallery
	default:
		return fmt.Errorf("unknown tag: %s", node.Tag)
	}
	return nil
}

func ReadWyWeb(dir string, forceTag ...string) (WyWebMeta, error) {
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	var filename string
	if stat.IsDir() {
		filename = filepath.Join(dir, "wyweb")
	} else {
		filename = dir
	}
	wywebData, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var meta Document
	if len(forceTag) > 0 {
		meta.forceTag = forceTag[0]
	}
	err = yaml.Unmarshal(wywebData, &meta)
	if err != nil {
		log.Println(string(wywebData))
		return nil, err
	}
	return meta.Data, nil
}
