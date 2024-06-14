package main

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
)

type Heritable struct {
	Author     string
	Copyright  string
	DomainName string
	Meta       []string
	Styles     []string
	Scripts    []string
}

type configNode struct {
	Children map[string]*configNode
	Parent   *configNode
	Data     *WyWebMeta
	Tree     *ConfigTree
	Resolved *Heritable
	Path     string
}

func newConfigNode() configNode {
	var out configNode
	out.Children = make(map[string]*configNode)
	out.Resolved = nil
	return out
}

type ConfigTree struct {
	Root    *configNode
	Scripts map[string]WWHeaderInclude
	Styles  map[string]WWHeaderInclude
	Domain  string
	mu      sync.Mutex
}

func (tree *ConfigTree) RegisterConfig(cfg *WyWebMeta) (*configNode, error) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	rootPath := PathToList(tree.Root.Path)
	thisPath := PathToList((*cfg).GetPath())
	_, err := NearestCommonAncestor(rootPath, thisPath)
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
	fmt.Println(reflect.TypeOf(meta))
	switch (*meta).(type) {
	case *WyWebRoot:
		temp := (*meta).(*WyWebRoot)
		node.Resolved = &Heritable{
			Author:     temp.Author,
			Copyright:  temp.Copyright,
			DomainName: temp.DomainName,
			Meta:       temp.Meta,
			Styles:     make([]string, len(temp.Default.Styles)),
			Scripts:    make([]string, len(temp.Default.Scripts)),
		}
		copy(node.Resolved.Styles, temp.Default.Styles)
		copy(node.Resolved.Scripts, temp.Default.Scripts)
		return nil
	case *WyWebPost, *WyWebListing, *WyWebGallery:
		if node.Parent.Resolved == nil {
			node.Parent.resolve()
		}
		//var author string
		//var copyright string
		head := (*meta).GetHeadData()
		page := (*meta).GetPageData()
		node.Resolved = &Heritable{
			Author:     node.Parent.Resolved.Author,
			Copyright:  node.Parent.Resolved.Copyright,
			DomainName: node.Parent.Resolved.DomainName,
			Meta:       node.Parent.Resolved.Meta,
			//Styles:     make([]string, 0),
			//Scripts:    make([]string, 0),
		}
		if !reflect.ValueOf(page.Author).IsZero() {
			node.Resolved.Author = page.Author
		}
		if !reflect.ValueOf(page.Copyright).IsZero() {
			node.Resolved.Copyright = page.Copyright
		}
		//for _, style := range node.Parent.Resolved.Styles {
		//	excluded := slices.Contains(temp.Exclude.Styles, style)
		//	present := slices.Contains(node.Resolved.Styles, style)
		//	if !excluded && !present {
		//		node.Resolved.Styles = append(node.Resolved.Styles, style)
		//	}
		//}
		//for _, style := range temp.Include.Styles {
		//	present := slices.Contains(node.Resolved.Styles, style)
		//	if !present {
		//		node.Resolved.Styles = append(node.Resolved.Styles, style)
		//	}
		//}
		localStyles := make([]string, 0)
		localScripts := make([]string, 0)
		for styleName, styleValue := range head.Styles {
			_, ok := node.Tree.Styles[styleName]
			if !ok {
				fmt.Printf("WARN: In configuration %s, the style %s is already defined. The new definition will be ignored.\n", node.Path, styleName)
			} else {
				node.Tree.Styles[styleName] = styleValue
				localStyles = append(localStyles, styleName)
			}
		}
		styleIncludes := ConcatUnique(head.Include.Styles, node.Parent.Resolved.Styles)
		node.Resolved.Styles, _ = includeExclude(localStyles, styleIncludes, head.Exclude.Styles)
		//for _, script := range node.Parent.Resolved.Scripts {
		//	excluded := slices.Contains(temp.Exclude.Scripts, script)
		//	present := slices.Contains(node.Resolved.Scripts, script)
		//	if !excluded && !present {
		//		node.Resolved.Scripts = append(node.Resolved.Scripts, script)
		//	}
		//}
		//for _, script := range temp.Include.Scripts {
		//	present := slices.Contains(node.Resolved.Scripts, script)
		//	if !present {
		//		node.Resolved.Scripts = append(node.Resolved.Scripts, script)
		//	}
		//}
		for scriptName, scriptValue := range head.Scripts {
			_, ok := node.Tree.Scripts[scriptName]
			if !ok {
				fmt.Printf("WARN: In configuration %s, the script %s is already defined. The new definition will be ignored.\n", node.Path, scriptName)
			} else {
				node.Tree.Scripts[scriptName] = scriptValue
				localScripts = append(localScripts, scriptName)
			}
		}
		scriptIncludes := ConcatUnique(head.Include.Scripts, node.Parent.Resolved.Scripts)
		node.Resolved.Scripts, _ = includeExclude(localScripts, scriptIncludes, head.Exclude.Scripts)
	default:
		fmt.Printf("Meta: %s\n", string(reflect.TypeOf(meta).Name()))
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
		meta, e := readWyWeb(path)
		if e != nil {
			fmt.Fprintf(os.Stderr, "%v\n", e)
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
	fmt.Printf("New child node:\n%+v\n", node)
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
		Domain:  domain,
		Root:    &rootnode,
		Scripts: make(map[string]WWHeaderInclude),
		Styles:  make(map[string]WWHeaderInclude),
	}
	rootnode.Tree = &out
	meta, err := readWyWeb(documentRoot)
	if err != nil {
		fmt.Printf("Document root: %s\n", documentRoot)
		return nil, err
	}
	if (meta).GetType() != "root" {
		return nil, fmt.Errorf("the wyweb file located at %s must be of type root", documentRoot)
	}
	for k, v := range (meta).(*WyWebRoot).Available.Styles {
		out.Styles[k] = v
	}
	for k, v := range (meta).(*WyWebRoot).Available.Scripts {
		out.Scripts[k] = v
	}
	rootnode.Data = &meta
	rootnode.growTree(documentRoot, &out)
	return &out, nil
}

func (tree *ConfigTree) search(path string) (*WyWebMeta, *Heritable, error) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	node := tree.Root
	var child *configNode
	var directory string
	thisPath := PathToList(path)
	for _, directory = range thisPath {
		child = node.Children[directory]
		if child == nil {
			break
		}
		node = child
	}
	fmt.Printf("End: %s | %s", directory, thisPath[len(thisPath)-1])
	// there are previously undiscovered directories that need to be filled in
	if directory != thisPath[len(thisPath)-1] {
		return nil, nil, fmt.Errorf("not found")
	}
	return node.Data, node.Resolved, nil
}
