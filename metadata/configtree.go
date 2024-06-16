package metadata

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"

	"wyweb.site/wyweb/util"
)

// A Distillate is the resolved and "distilled" data about a given webpage. It is the result of determining all includes
// and excludes, as well as the final rendered HTML of the page.
type Distillate struct {
	HTML       *[]byte
	Author     string
	Copyright  string
	DomainName string
	Meta       []string // The actual html <meta> elements as text.
	Resources  []string // the names (keys) of resources requested by this page
}

type configNode struct {
	Children map[string]*configNode
	Parent   *configNode
	Data     *WyWebMeta
	Tree     *ConfigTree
	Resolved *Distillate
	Path     string
}

func newConfigNode() configNode {
	var out configNode
	out.Children = make(map[string]*configNode)
	out.Resolved = nil
	return out
}

// Logical representation of the entire website
type ConfigTree struct {
	Root      *configNode
	Resources map[string]Resource
	Domain    string
	mu        sync.Mutex
}

// Create a new configNode from cfg and add it to the tree
func (tree *ConfigTree) RegisterConfig(cfg *WyWebMeta) (*configNode, error) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	rootPath := util.PathToList(tree.Root.Path)
	thisPath := util.PathToList((*cfg).GetPath())
	_, err := util.NearestCommonAncestor(rootPath, thisPath)
	if err != nil {
		return nil, err
	}
	parent := tree.Root
	var child *configNode
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
	result := configNode{
		Path:     (*cfg).GetPath(),
		Parent:   parent,
		Data:     cfg,
		Children: make(map[string]*configNode),
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

func (node *configNode) resolve() error {
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
			Author:     temp.Author,
			Copyright:  temp.Copyright,
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
			Author:     node.Parent.Resolved.Author,
			Copyright:  node.Parent.Resolved.Copyright,
			DomainName: node.Parent.Resolved.DomainName,
			Meta:       node.Parent.Resolved.Meta,
			HTML:       nil,
		}
		if !reflect.ValueOf(page.Author).IsZero() {
			node.Resolved.Author = page.Author
		}
		if !reflect.ValueOf(page.Copyright).IsZero() {
			node.Resolved.Copyright = page.Copyright
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
	return nil
}

func (node *configNode) growTree(dir string, tree *ConfigTree) error {
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
			return nil
		}
		child := configNode{
			Path:     path,
			Parent:   node,
			Data:     &meta,
			Children: make(map[string]*configNode),
			Tree:     tree,
			Resolved: nil,
		}
		node.Children[filepath.Base(path)] = &child
		child.resolve()
		//child, _ := tree.RegisterConfig(meta)
		child.growTree(path, tree)
		return nil
	})
	return status
}

func BuildConfigTree(documentRoot string, domain string) (*ConfigTree, error) {
	var err error
	rootnode := configNode{
		Path:     documentRoot,
		Parent:   nil,
		Data:     nil,
		Children: make(map[string]*configNode),
		Tree:     nil,
		Resolved: nil,
	}
	out := ConfigTree{
		Domain:    domain,
		Root:      &rootnode,
		Resources: make(map[string]Resource),
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

func (tree *ConfigTree) Search(path string) (*WyWebMeta, *Distillate, error) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	node := tree.Root
	var child *configNode
	var directory string
	thisPath := util.PathToList(path)[1:]
	for _, directory = range thisPath {
		child = node.Children[directory]
		if child == nil {
			break
		}
		node = child
	}
	// there are previously undiscovered directories that need to be filled in
	if node.Path != path {
		fmt.Printf("%+v\n", *node)
		return nil, nil, fmt.Errorf("not found")
	}
	//log.Printf("%s: %+v\n\n", path, node.Resolved)
	return node.Data, node.Resolved, nil
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
