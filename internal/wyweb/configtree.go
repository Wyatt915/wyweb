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
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"wyweb.site/util"
)

// Logical representation of the entire website
type ConfigTree struct {
	Root         *ConfigNode
	TagDB        map[string][]Listable
	Resources    map[string]Resource
	DocumentRoot string
	Domain       string
	sync.RWMutex
}

func (tree *ConfigTree) GetResource(name string) (Resource, bool) {
	tree.RLock()
	defer tree.RUnlock()
	res, ok := tree.Resources[name]
	return res, ok
}

func (tree *ConfigTree) SetResource(name string, res Resource) {
	tree.Lock()
	defer tree.Unlock()
	tree.Resources[name] = res
}

// Create a new configNode from cfg and add it to the tree
func (tree *ConfigTree) RegisterConfig(cfg *WyWebMeta) (*ConfigNode, error) {
	tree.Lock()
	defer tree.Unlock()
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
		Parent:   parent,
		Data:     cfg,
		Children: make(map[string]*ConfigNode),
		TagDB:    make(map[string][]Listable),
		Tree:     tree,
	}
	result.Path = (*cfg).GetPath()
	parent.Children[directory] = &result
	result.resolve()
	return &result, nil
}

func createNodeFromPath(parent *ConfigNode, dir string, file os.DirEntry) (*ConfigNode, error) {
	filename := file.Name()
	ignore := []string{".git"}
	if slices.Contains(ignore, filename) {
		return nil, fmt.Errorf("ignoring %s", filename)
	}
	var meta WyWebMeta
	var e error
	path := filepath.Join(dir, filename)
	var child *ConfigNode
	if !file.IsDir() {
		if strings.HasSuffix(filename, ".post.md") {
			child = MagicPost(parent, filename)
		} else {
			return nil, fmt.Errorf("%s could not be interpreted as a WyWeb page", filename)
		}
	} else {
		wwFileName := filepath.Join(dir, filename, "wyweb")
		_, e = os.Stat(wwFileName)
		if e != nil {
			if strings.HasSuffix(filename, ".listing") || strings.ToLower(filename) == "blog" {
				child = MagicListing(parent, filename)
			} else {
				return nil, fmt.Errorf("%s could not be interpreted as a WyWeb page", filename)
			}
		} else {
			meta, e = ReadWyWeb(path)
			if e != nil {
				return nil, fmt.Errorf("couldn't read %s", path)
			}
			child = &ConfigNode{
				Parent:       parent,
				Data:         &meta,
				Children:     make(map[string]*ConfigNode),
				TagDB:        make(map[string][]Listable),
				Dependencies: make(map[string]DependencyKind),
				Tree:         parent.Tree,
			}
			(*child).Path = path
			(*child).Dependencies[wwFileName] = KindWyWeb
		}
	}
	return child, nil
}

func (node *ConfigNode) growTree(dir string, tree *ConfigTree) error {
	//node.Lock()
	//defer node.Unlock()
	var status error
	node.Tree = tree
	//filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	newNodeCreated := false
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		key := util.TrimMagicSuffix(path)
		info, err := file.Info()
		if err != nil {
			continue
		}
		if slices.Contains(node.knownFiles, file.Name()) && node.LastRead.After(info.ModTime()) {
			n, ok := node.Children[filepath.Base(key)]
			if ok {
				node.LastRead = time.Now()
				n.resolve()
				n.growTree(path, tree)
				continue
			}
		}
		node.knownFiles = append(node.knownFiles, file.Name())
		child, err := createNodeFromPath(node, dir, file)
		if err != nil {
			continue
		}
		if child == nil {
			continue
		}
		err = child.resolve()
		if err != nil {
			log.Println(err.Error())
			continue
		}
		node.Children[filepath.Base(key)] = child
		child.growTree(path, tree)
		newNodeCreated = true
		log.Println("NEW PAGE: ", child.Title)
	}
	if newNodeCreated {
		setNavLinksOfChildren(node)
		if node.NodeKind == WWLISTING {
			node.HTML = nil
		}
	} else {
		status = fmt.Errorf("no new files found")
	}
	return status
}

func BuildConfigTree(documentRoot string, domain string) (*ConfigTree, error) {
	var err error
	rootnode := ConfigNode{
		Parent:       nil,
		Data:         nil,
		Children:     make(map[string]*ConfigNode),
		Tree:         nil,
		TagDB:        make(map[string][]Listable),
		Dependencies: make(map[string]DependencyKind),
	}
	out := ConfigTree{
		Domain:       domain,
		DocumentRoot: documentRoot,
		Root:         &rootnode,
		Resources:    make(map[string]Resource),
		TagDB:        make(map[string][]Listable),
	}
	rootnode.Tree = &out
	meta, err := ReadWyWeb(documentRoot)
	if err != nil {
		log.Printf("Document root: %s\n", documentRoot)
		return nil, err
	}
	if (meta).GetType() != WWROOT {
		return nil, fmt.Errorf("the wyweb file located at %s must be of type root", documentRoot)
	}
	for k, v := range (meta).(*WyWebRoot).Resources {
		out.Resources[k] = v
	}
	rootnode.Data = &meta
	rootnode.growTree(documentRoot, &out)
	//for tag, lst := range out.TagDB {
	//	fmt.Printf("\n%s:\n\t", tag)
	//	for _, item := range lst {
	//		fmt.Printf("%s, ", item.GetTitle())
	//	}
	//}
	out.MakeSitemap()
	out.MakeRSS()
	go out.watchForDependencyChanges(time.Second)
	return &out, nil
}

func (node *ConfigNode) search(path []string, idx int) (*ConfigNode, error) {
	//node.RLock()
	//defer node.RUnlock()
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
	tree.RLock()
	defer tree.RUnlock()
	node := tree.Root
	pathList := util.PathToList(path)
	return node.search(pathList, 0)
}

func (tree *ConfigTree) GetItemsByTag(tag string) []Listable {
	tree.RLock()
	defer tree.RUnlock()
	return tree.TagDB[tag]
}

func (node *ConfigNode) GetItemsByTag(tag string) []Listable {
	//node.RLock()
	//defer node.RUnlock()
	return node.TagDB[tag]
}

func (tree *ConfigTree) GetDefaultHead() *HTMLHeadData {
	tree.RLock()
	defer tree.RUnlock()
	out := tree.Root.GetHTMLHeadData()
	(*out).Title = ""
	return out
}

func (node *ConfigNode) printTree(level int) {
	for range level {
		print("    ")
	}
	fmt.Printf("%s\t(%s - %s)\n", node.Title, KindNames[node.NodeKind], node.Path)
	for _, child := range node.Children {
		child.printTree(level + 1)
	}
}

func watchRecurse(node *ConfigNode) {
	needsUpdate := make([]string, 0)
	needsRemoval := make([]string, 0)
	staleIDs := make(map[string]string)
	for key, child := range node.Children {
		modifiedDep := false
		for path, kind := range child.Dependencies {
			st, err := os.Stat(path)
			if errors.Is(err, os.ErrNotExist) && (kind == KindWyWeb || kind == KindMDSource) {
				log.Println("REMOVING ", child.Title)
				staleIDs[key] = child.GetIDb64()
				needsRemoval = append(needsRemoval, key)
				break
			}
			if err != nil {
				log.Println(err.Error())
				continue
			}
			if st.ModTime().After(child.LastRead.Add(time.Second)) {
				modifiedDep = true
				break
			}
		}
		if modifiedDep {
			staleIDs[key] = child.GetIDb64()
			needsUpdate = append(needsUpdate, key)
		}
	}
	for _, deadNode := range needsRemoval {
		delete(node.Children, deadNode)
	}
	for _, staleNode := range needsUpdate {
		tempChildMap := make(map[string]*ConfigNode)
		for k, v := range node.Children[staleNode].Children {
			tempChildMap[k] = v
		}
		path := node.Children[staleNode].RealPath
		dir := filepath.Dir(path)
		st, _ := os.Stat(path)
		file := fs.FileInfoToDirEntry(st)
		newChild, err := createNodeFromPath(node, dir, file)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		delete((*node).Children, staleNode)
		err = newChild.resolve()
		if err != nil {
			log.Println("ERROR: ", err.Error())
		}
		log.Printf("UPDATED %s", newChild.Title)
		(*newChild).Children = tempChildMap
		for key := range newChild.Children {
			newChild.Children[key].Parent = newChild
		}
		t := time.Now()
		newChild.LastRead = t
		node.Children[staleNode] = newChild
	}
	if len(needsUpdate) > 0 || len(needsRemoval) > 0 {
		setNavLinksOfChildren(node)
	}
	for _, child := range node.Children {
		watchRecurse(child)
	}
	if node.NodeKind == WWLISTING {
		for _, staleNode := range needsUpdate {
			oldlisting, err := node.HTML.GetElementByID(staleIDs[staleNode])
			if err != nil {
				continue
			}
			*oldlisting = *postToListItem(node.Children[staleNode])
		}
		for _, deadNode := range needsRemoval {
			oldlisting, err := node.HTML.GetElementByID(staleIDs[deadNode])
			if err != nil {
				continue
			}
			node.HTML.RemoveNode(oldlisting)
		}
	}
}

func (tree *ConfigTree) watchForDependencyChanges(frequency time.Duration) {
	for {
		watchRecurse(tree.Root)
		tree.Root.growTree(filepath.Base(tree.DocumentRoot), tree)
		time.Sleep(frequency)
	}
}

// explicit refers to dates explicitly given by the author, whereas implicit refers to any automatically updated times.
// implicit datetimes will most often be created by the watchForDependencyChanges function.
func (node *ConfigNode) getMostRecentDates() (explicit, implicit time.Time) {
	var ex, im time.Time
	var dft func(*ConfigNode)
	dft = func(node *ConfigNode) {
		if ex.IsZero() || node.Date.After(ex) {
			ex = node.Date
		}
		if node.Updated.After(ex) {
			ex = node.Updated
		}
		if node.LastRead.After(im) {
			im = node.LastRead
		}
		for _, child := range node.Children {
			dft(child)
		}
	}
	dft(node)
	return ex, im
}
