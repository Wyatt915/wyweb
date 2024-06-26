package metadata

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"wyweb.site/wyweb/html"
	"wyweb.site/wyweb/util"
)

// A Distillate is the resolved and "distilled" data about a given webpage. It is the result of determining all includes
// and excludes, as well as the final rendered HTML of the page.
type Distillate struct {
	PageData   PageData
	HTML       *html.HTMLElement
	DomainName string
	Meta       []string // The actual html <meta> elements as text.
	Resources  []string // the names (keys) of resources requested by this page
}

type ConfigNode struct {
	Children map[string]*ConfigNode
	Parent   *ConfigNode
	Data     *WyWebMeta
	Tree     *ConfigTree
	Resolved *Distillate
	Path     string
	Date     time.Time
}

type Listable interface {
	GetDate() time.Time
}

func (n *ConfigNode) GetDate() time.Time {
	return n.Date
}

func newConfigNode() ConfigNode {
	var out ConfigNode
	out.Children = make(map[string]*ConfigNode)
	out.Resolved = nil
	return out
}

// Logical representation of the entire website
type ConfigTree struct {
	Root      *ConfigNode
	TagDB     map[string][]Listable
	Resources map[string]Resource
	Domain    string
	mu        sync.Mutex
}

// Create a new configNode from cfg and add it to the tree
func (tree *ConfigTree) RegisterConfig(cfg *WyWebMeta) (*ConfigNode, error) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	rootPath := util.PathToList(tree.Root.Path)
	thisPath := util.PathToList((*cfg).GetPath())
	_, err := util.NearestCommonAncestor(rootPath, thisPath)
	if err != nil {
		return nil, err
	}
	parent := tree.Root
	var child *ConfigNode
	var directory string
	keepSearching := true
	for keepSearching {
		var idx int
		for idx, directory = range thisPath {
			child = parent.Children[directory]
			if child == nil {
				break
			}
			parent = child
		}
		// there are previously undiscovered directories that need to be filled in
		if directory != thisPath[len(thisPath)-1] {
			child.growTree(strings.Join(thisPath[idx:], string(os.PathSeparator)), tree)
		} else {
			keepSearching = false
		}
	}
	if parent.Children[directory] != nil {
		return parent.Children[directory], nil
	}
	result := ConfigNode{
		Path:     (*cfg).GetPath(),
		Parent:   parent,
		Data:     cfg,
		Children: make(map[string]*ConfigNode),
		Tree:     tree,
		Resolved: nil,
	}
	parent.Children[directory] = &result
	result.resolve()
	return &result, nil
}

// includeExclude resolves which includables (styles or scripts) will be used on the page.
// local - the new includables of the cueent node
// include - the in
func includeExclude(local []string, include []string, exclude []string) ([]string, error) {
	result := make([]string, 0)
	for _, name := range local {
		if slices.Contains(exclude, name) {
			log.Printf("WARN: The locally defined %s, is already defined and set to be excluded. The new definition will be ignored.\n", name)
		} else if slices.Contains(include, name) {
			log.Printf("WARN: The locally defined %s, is already defined. The new definition will be ignored.\n", name)
		} else {
			result = append(result, name)
		}
	}
	for _, name := range include {
		exists := slices.Contains(local, name)
		excluded := slices.Contains(exclude, name)
		if !exists && !excluded {
			result = append(result, name)
		}
	}
	return result, nil
}

func (node *ConfigNode) resolve() error {
	if node.Resolved != nil {
		return nil
	}
	meta := node.Data
	if meta == nil {
		return nil
	}
	switch (*meta).(type) {
	case *WyWebRoot:
		temp := (*meta).(*WyWebRoot)
		node.Resolved = &Distillate{
			PageData:   *temp.GetPageData(),
			DomainName: temp.DomainName,
			Meta:       temp.Meta,
			Resources:  make([]string, len(temp.Default.Resources)),
			HTML:       nil,
		}
		copy(node.Resolved.Resources, temp.Default.Resources)
		return nil
	case *WyWebPost, *WyWebListing, *WyWebGallery:
		if node.Parent.Resolved == nil {
			node.Parent.resolve()
		}
		//var author string
		//var copyright string
		head := (*meta).GetHeadData()
		page := (*meta).GetPageData()
		node.Resolved = &Distillate{
			PageData:   *page,
			DomainName: node.Parent.Resolved.DomainName,
			Meta:       node.Parent.Resolved.Meta,
			HTML:       nil,
		}
		//		node.Resolved.PageData = *page
		if node.Resolved.PageData.Author == "" {
			node.Resolved.PageData.Author = node.Parent.Resolved.PageData.Author
		}
		if node.Resolved.PageData.Copyright == "" {
			node.Resolved.PageData.Copyright = node.Parent.Resolved.PageData.Copyright
		}
		local := make([]string, 0)
		for name, value := range head.Resources {
			_, ok := node.Tree.Resources[name]
			if !ok {
				log.Printf("WARN: In configuration %s, the Resource %s is already defined. The new definition will be ignored.\n", node.Path, name)
			} else {
				node.Tree.Resources[name] = value
				local = append(local, name)
			}
		}
		includes := util.ConcatUnique(head.Include, node.Parent.Resolved.Resources)
		excludes := (*node.Parent.Data).GetHeadData().Exclude
		// any excludes of the parent are overridden by local includes. Otherwise, they are inherited.
		n := 0
		for _, x := range excludes {
			if !slices.Contains(head.Include, x) {
				excludes[n] = x
				n++
			}
		}
		excludes = util.ConcatUnique(excludes[:n], head.Exclude)
		node.Resolved.Resources, _ = includeExclude(local, includes, excludes)
	default:
		log.Printf("Meta: %s\n", string(reflect.TypeOf(meta).Name()))
	}
	//register tags
	tree := node.Tree
	switch t := (*meta).(type) {
	case *WyWebPost:
		for _, tag := range t.Tags {
			if _, ok := tree.TagDB[tag]; !ok {
				tree.TagDB[tag] = make([]Listable, 0)
			}
			//if !slices.Contains(tree.TagDB[tag], node) {
			tree.TagDB[tag] = append(tree.TagDB[tag], node)
			//}
		}
	case *WyWebGallery:
		for idx, item := range t.GalleryItems {
			t.GalleryItems[idx].GalleryPath = t.Path
			for _, tag := range item.Tags {
				if _, ok := tree.TagDB[tag]; !ok {
					tree.TagDB[tag] = make([]Listable, 0)
				}
				tree.TagDB[tag] = append(tree.TagDB[tag], item)
			}

		}
	}
	return nil
}

func setNavLink(nl *WWNavLink, path, title string) {
	if nl.Path == "" {
		nl.Path = path
	}
	if nl.Text == "" {
		nl.Text = title
	}
}

// If NavLinks are not explicitly defined, set them by ordering items by creation date
func setNavLinksOfChildren(node *ConfigNode) {
	siblings := make([]*ConfigNode, len(node.Children))
	i := 0
	for _, child := range node.Children {
		siblings[i] = child
		i++
	}
	sort.Slice(siblings, func(i, j int) bool {
		return siblings[i].Date.Before(siblings[j].Date)
	})
	var path, text string
	for i, obj := range siblings {
		res := obj.Resolved
		if i > 0 {
			path = "/" + siblings[i-1].Path
			text = siblings[i-1].Resolved.PageData.Title
			setNavLink(&res.PageData.Prev, path, text)
		}
		path, _ = filepath.Rel(".", node.Path)
		path = "/" + path
		text = node.Resolved.PageData.Title
		setNavLink(&res.PageData.Up, path, text)
		if i < len(siblings)-1 {
			path = "/" + siblings[i+1].Path
			text = siblings[i+1].Resolved.PageData.Title
			setNavLink(&res.PageData.Next, path, text)
		}
	}
}

func (node *ConfigNode) growTree(dir string, tree *ConfigTree) error {
	var status error
	node.Tree = tree
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		wwFileName := filepath.Join(path, "wyweb")
		_, e := os.Stat(wwFileName)
		if e != nil {
			return nil
		}
		meta, e := ReadWyWeb(path)
		if e != nil {
			log.Printf("couldn't read %s", path)
			return nil
		}
		child := ConfigNode{
			Path:     path,
			Parent:   node,
			Data:     &meta,
			Children: make(map[string]*ConfigNode),
			Tree:     tree,
			Resolved: nil,
			Date:     meta.GetPageData().Date,
		}
		node.Children[filepath.Base(path)] = &child
		child.resolve()
		child.growTree(path, tree)
		return nil
	})
	setNavLinksOfChildren(node)
	return status
}

func BuildConfigTree(documentRoot string, domain string) (*ConfigTree, error) {
	var err error
	rootnode := ConfigNode{
		Path:     documentRoot,
		Parent:   nil,
		Data:     nil,
		Children: make(map[string]*ConfigNode),
		Tree:     nil,
		Resolved: nil,
	}
	out := ConfigTree{
		Domain:    domain,
		Root:      &rootnode,
		Resources: make(map[string]Resource),
		TagDB:     make(map[string][]Listable),
	}
	rootnode.Tree = &out
	meta, err := ReadWyWeb(documentRoot)
	if err != nil {
		log.Printf("Document root: %s\n", documentRoot)
		return nil, err
	}
	if (meta).GetType() != "root" {
		return nil, fmt.Errorf("the wyweb file located at %s must be of type root", documentRoot)
	}
	for k, v := range (meta).(*WyWebRoot).Resources {
		out.Resources[k] = v
	}
	rootnode.Data = &meta
	rootnode.growTree(documentRoot, &out)
	return &out, nil
}

func (node *ConfigNode) search(path []string, idx int) (*ConfigNode, error) {
	// we have SUCCESSFULLY reached the end of the path
	if len(path) == idx {
		return node, nil
	}
	child, ok := node.Children[path[idx]]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return child.search(path, idx+1)
}

func (tree *ConfigTree) Search(path string) (*ConfigNode, error) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	node := tree.Root
	log.Printf("Searching For %s\n", path)
	pathList := util.PathToList(path)
	return node.search(pathList, 0)
}

func (tree *ConfigTree) GetItemsByTag(tag string) []Listable {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	return tree.TagDB[tag]
}

type URLResource struct {
	String     string
	Attributes map[string]string
}

type RawResource struct {
	String     string
	Attributes map[string]string
}

type HTMLHeadData struct {
	Title   string
	Meta    []string
	Styles  []interface{}
	Scripts []interface{}
}

func (tree *ConfigTree) GetDefaultHead() *HTMLHeadData {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	resolved := tree.Root.Resolved
	styles := make([]interface{}, 0)
	scripts := make([]interface{}, 0)
	for _, name := range resolved.Resources {
		res, ok := tree.Resources[name]
		if !ok {
			log.Printf("%s does not exist in the resource registry.\n", name)
			continue
		}
		var value interface{}
		switch res.Method {
		case "raw":
			value = RawResource{String: res.Value, Attributes: res.Attributes}
		case "url":
			value = URLResource{String: res.Value, Attributes: res.Attributes}
		default:
			log.Printf("Unknown method for resource %s: %s\n", name, res.Type)
		}
		switch res.Type {
		case "style":
			styles = append(styles, value)
		case "script":
			scripts = append(scripts, value)
		default:
			log.Printf("Unknown type for resource %s: %s\n", name, res.Type)
		}
	}
	out := &HTMLHeadData{
		Styles:  styles,
		Scripts: scripts,
	}
	return out
}

func (tree *ConfigTree) GetHeadData(meta *WyWebMeta, resolved *Distillate) *HTMLHeadData {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	page := (*meta).GetPageData()
	styles := make([]interface{}, 0)
	scripts := make([]interface{}, 0)
	for _, name := range resolved.Resources {
		res, ok := tree.Resources[name]
		if !ok {
			log.Printf("%s does not exist in the resource registry.\n", name)
			continue
		}
		var value interface{}
		switch res.Method {
		case "raw":
			value = RawResource{String: res.Value, Attributes: res.Attributes}
		case "url":
			value = URLResource{String: res.Value, Attributes: res.Attributes}
		default:
			log.Printf("Unknown method for resource %s: %s\n", name, res.Type)
		}
		switch res.Type {
		case "style":
			styles = append(styles, value)
		case "script":
			scripts = append(scripts, value)
		default:
			log.Printf("Unknown type for resource %s: %s\n", name, res.Type)
		}
	}
	out := &HTMLHeadData{
		Title:   page.Title,
		Meta:    resolved.Meta,
		Styles:  styles,
		Scripts: scripts,
	}
	return out
}
