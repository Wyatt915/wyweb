package main

import (
	"fmt"
	"io/fs"
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
	Meta       string
	Styles     []string
	Scripts    []string
}

type configNode struct {
	Path     string
	Parent   *configNode
	Data     *WyWebMeta
	Children map[string]*configNode
	Tree     *ConfigTree
	Resolved *Heritable
}

func newConfigNode() configNode {
	var out configNode
	out.Children = make(map[string]*configNode)
	return out
}

type ConfigTree struct {
	mu      sync.Mutex
	Domain  string
	Root    *configNode
	Scripts map[string]WWHeaderInclude
	Styles  map[string]WWHeaderInclude
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
	result := newConfigNode()
	result.Path = (*cfg).GetPath()
	result.Parent = parent
	result.Tree = tree
	result.Data = cfg
	parent.Children[directory] = &result
	result.resolve()
	return &result, nil
}

func (node *configNode) resolve() error {
	if node.Resolved != nil {
		return nil
	}
	node.Resolved = new(Heritable)
	meta := *node.Data
	switch meta.(type) {
	case WyWebRoot:
		temp := meta.(*WyWebRoot)
		*node.Resolved = Heritable{
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
	case WyWebPage, WyWebPost, WyWebListing, WyWebGallery:
		tempPtr, _ := AsPage(&meta)
		temp := *tempPtr
		if node.Parent.Resolved == nil {
			node.Parent.resolve()
		}
		*node.Resolved = Heritable{
			Author:     node.Parent.Resolved.Author,
			Copyright:  node.Parent.Resolved.Copyright,
			DomainName: node.Parent.Resolved.DomainName,
			Meta:       node.Parent.Resolved.Meta,
			Styles:     make([]string, 0),
			Scripts:    make([]string, 0),
		}
		if !reflect.ValueOf(temp.Author).IsZero() {
			node.Resolved.Author = temp.Author
		}
		if !reflect.ValueOf(temp.Copyright).IsZero() {
			node.Resolved.Copyright = temp.Copyright
		}
		for _, style := range node.Parent.Resolved.Styles {
			excluded := slices.Contains(temp.Exclude.Styles, style)
			present := slices.Contains(node.Resolved.Styles, style)
			if !excluded && !present {
				node.Resolved.Styles = append(node.Resolved.Styles, style)
			}
		}
		for _, style := range temp.Include.Styles {
			present := slices.Contains(node.Resolved.Styles, style)
			if !present {
				node.Resolved.Styles = append(node.Resolved.Styles, style)
			}
		}
		for styleName, styleValue := range temp.Styles {
			_, ok := node.Tree.Styles[styleName]
			if !ok {
				fmt.Printf("WARN: In configuration %s, the style %s is already defined. The new definition will be ignored.\n", node.Path, styleName)
			} else {
				node.Tree.Styles[styleName] = styleValue
			}
		}
		for _, script := range node.Parent.Resolved.Scripts {
			excluded := slices.Contains(temp.Exclude.Scripts, script)
			present := slices.Contains(node.Resolved.Scripts, script)
			if !excluded && !present {
				node.Resolved.Scripts = append(node.Resolved.Scripts, script)
			}
		}
		for _, script := range temp.Include.Scripts {
			present := slices.Contains(node.Resolved.Scripts, script)
			if !present {
				node.Resolved.Scripts = append(node.Resolved.Scripts, script)
			}
		}
		for scriptName, scriptValue := range temp.Scripts {
			_, ok := node.Tree.Scripts[scriptName]
			if !ok {
				fmt.Printf("WARN: In configuration %s, the script %s is already defined. The new definition will be ignored.\n", node.Path, scriptName)
			} else {
				node.Tree.Scripts[scriptName] = scriptValue
			}
		}
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
		child := newConfigNode()
		child.Path = path
		child.Parent = node
		child.Data = &meta
		node.Children[filepath.Base(path)] = &child
		child.resolve()
		child.growTree(path, tree)
		return nil
	})

	return status
}

func BuildConfigTree(documentRoot string, domain string) (*ConfigTree, error) {
	var out ConfigTree
	var err error
	out.Domain = domain
	out.Styles = make(map[string]WWHeaderInclude)
	out.Scripts = make(map[string]WWHeaderInclude)
	out.Root = new(configNode)
	out.Root.Path = documentRoot
	out.Root.Data = new(WyWebMeta)
	out.Root.Children = make(map[string]*configNode)
	meta, err := readWyWeb(documentRoot)
	if err != nil {
		fmt.Printf("Document root: %s\n", documentRoot)
		return nil, err
	}
	if meta.GetType() != "root" {
		return nil, fmt.Errorf("the wyweb file located at %s must be of type root", documentRoot)
	}
	for k, v := range meta.(*WyWebRoot).Available.Styles {
		out.Styles[k] = v
	}
	for k, v := range meta.(*WyWebRoot).Available.Scripts {
		out.Scripts[k] = v
	}
	out.Root.Data = &meta
	out.Root.growTree(documentRoot, &out)
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
	// there are previously undiscovered directories that need to be filled in
	if directory != thisPath[len(thisPath)-1] {
		return nil, nil, fmt.Errorf("not found")
	}
	return node.Data, node.Resolved, nil
}
