package wyweb

import (
	"fmt"
	"log"
	"math/bits"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark/ast"

	"wyweb.site/util"
)

type ConfigNode struct {
	sync.RWMutex
	PageData
	HeadData
	// An id is constructed from the Date, Updated, and Resolved.Title fields.
	// The first 16 bits count the number of days after 2000-01-01 the item was published.
	// The next 8 bits count the number of days since the last update at the time the id was computed.
	// The final 40 bits are the concatenation of UTF-8 codepoints (with leading zeroes removed) taken from
	// Resolved.Titile.
	id             uint64
	resolved       bool
	NodeKind       WWNodeKind
	HTML           *HTMLElement
	Children       map[string]*ConfigNode
	LocalResources []string
	Data           *WyWebMeta
	Parent         *ConfigNode
	Index          string
	TagDB          map[string][]Listable
	Tags           []string
	Tree           *ConfigTree
	ParsedDocument *ast.Node
	Preview        string
	RealPath       string
	Images         []RichImage
	StructuredData []string
}

type Listable interface {
	GetDate() time.Time
	GetID() uint64
	GetTitle() string
	SetID()
}

func (n *ConfigNode) GetDate() time.Time {
	n.RLock()
	defer n.RUnlock()
	return n.Date
}
func (n *ConfigNode) GetID() uint64 {
	n.RLock()
	defer n.RUnlock()
	if n.id == 0 {
		n.RUnlock()
		n.SetID()
		n.RLock()
	}
	return n.id
}
func (n *ConfigNode) GetTitle() string {
	n.RLock()
	defer n.RUnlock()
	return n.Title
}

func (n *ConfigNode) GetHeadData() HeadData {
	n.RLock()
	defer n.RUnlock()
	return n.HeadData
}
func (n *ConfigNode) GetPageData() PageData {
	n.RLock()
	defer n.RUnlock()
	return n.PageData
}

func (n *ConfigNode) SetID() {
	n.Lock()
	defer n.Unlock()
	epoch := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	age := uint64((n.Date.Sub(epoch)).Hours()/24) & 0xFFFF // 16 bits will last until June 2179
	var sinceUpdate uint64
	temp := time.Since(n.Updated).Hours() / 24
	if temp > 255 {
		sinceUpdate = 255
	} else {
		sinceUpdate = uint64(temp) & 0xFF // 8 bits corresponds to ~ 8 months
	}
	//fmt.Printf("%016b\n", age)
	//fmt.Printf("%s%08b\n", strings.Repeat(" ", 16), sinceUpdate)
	n.id = (age << (64 - 16)) | (sinceUpdate << (64 - 16 - 8))
	bitsRemaining := 64 - (16 + 8)
	for _, char := range n.Title {
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

func newConfigNode() *ConfigNode {
	var out *ConfigNode
	out.Children = make(map[string]*ConfigNode)
	out.TagDB = make(map[string][]Listable)
	return out
}

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

// includeExclude resolves which includables (styles or scripts) will be used on the page.
// local - the new includables of the cueent node
// include - the in
func includeExclude(local []string, include []string, exclude []string) []string {
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
	return result
}
func (tree *ConfigTree) resolveResourceDeps(res []string) []string {
	names := make([]string, len(res))
	copy(names, res)
	keepGoing := true
	for keepGoing {
		keepGoing = false
		for i := 0; i < len(names); i++ {
			for _, dep := range tree.Resources[names[i]].DependsOn {
				if !slices.Contains(names, dep) {
					keepGoing = true
					names = append(names, dep)
				}
			}
		}
	}
	return names
}

func regiserTag(tag string, item Listable, tagdb *map[string][]Listable) {
	if _, ok := (*tagdb)[tag]; !ok {
		(*tagdb)[tag] = make([]Listable, 0)
		(*tagdb)[tag] = append((*tagdb)[tag], item)
		return
	}
	doAppend := !slices.ContainsFunc((*tagdb)[tag], func(l Listable) bool {
		return l.GetID() == item.GetID()
	})
	if doAppend {
		(*tagdb)[tag] = append((*tagdb)[tag], item)
	}
}

func copyHeadData(dest *HeadData, src *HeadData) {
	dest.Include = make([]string, len(src.Include))
	dest.Exclude = make([]string, len(src.Exclude))
	dest.Meta = make([]string, len(src.Meta))
	copy(dest.Include, src.Include)
	copy(dest.Exclude, src.Exclude)
	copy(dest.Meta, src.Meta)
	dest.Resources = make(map[string]Resource)
	for k, v := range src.Resources {
		dest.Resources[k] = v
	}
}

func (node *ConfigNode) resolveIncludes() {
	node.LocalResources = make([]string, 0)
	local := make([]string, 0)
	for name, value := range node.Resources {
		_, ok := node.Tree.Resources[name]
		if ok {
			log.Printf("WARN: In configuration %s, the Resource %s is already defined. The new definition will be ignored.\n", node.Path, name)
			continue
		}
		node.Tree.Resources[name] = value
		local = append(local, name)
	}
	includes := util.ConcatUnique(node.Include, node.Parent.LocalResources)
	excludes := make([]string, len(node.Parent.Exclude))
	copy(excludes, node.Parent.Exclude)
	// any excludes of the parent are overridden by local includes. Otherwise, they are inherited.
	n := 0
	for _, x := range excludes {
		if !slices.Contains(node.Include, x) {
			excludes[n] = x
			n++
		}
	}
	excludes = util.ConcatUnique(excludes[:n], node.Exclude)
	node.LocalResources = node.Tree.resolveResourceDeps(includeExclude(local, includes, excludes))
}

func (node *ConfigNode) inheritIfUndefined() {
	if node.Updated.IsZero() {
		node.Updated = node.Date
	}
	if node.Author == "" {
		node.Author = node.Parent.Author
	}
	if node.Copyright == "" {
		node.Copyright = node.Parent.Copyright
	}
}

// copy fields from src to dst only if the corresponding field of dst is zero/empty.
func mergePageData(dst *PageData, src *PageData) {
	if dst.Author == "" {
		dst.Author = src.Author
	}
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Description == "" {
		dst.Description = src.Description
	}
	if dst.Copyright == "" {
		dst.Copyright = src.Copyright
	}
	if dst.Path == "" {
		dst.Path = src.Path
	}
	if dst.ParentPath == "" {
		dst.ParentPath = src.ParentPath
	}
	if dst.Date.IsZero() {
		dst.Date = src.Date
	}
	if dst.Updated.IsZero() {
		dst.Updated = src.Updated
	}
	if dst.Next.IsZero() {
		dst.Next = src.Next
	}
	if dst.Prev.IsZero() {
		dst.Prev = src.Prev
	}
	if dst.Up.IsZero() {
		dst.Up = src.Up
	}
}

func (node *ConfigNode) SetFieldsFromWyWebMeta(meta *WyWebMeta) error {
	if node.NodeKind == WWNULL {
		node.NodeKind = (*meta).GetType()
	}
	switch (*meta).(type) {
	case *WyWebRoot:
		temp := (*meta).(*WyWebRoot)
		node.PageData = *temp.GetPageData()
		copyHeadData(&node.HeadData, temp.GetHeadData())
		node.LocalResources = make([]string, len(temp.Default.Resources))
		copy(node.LocalResources, temp.Default.Resources)
		node.resolved = true
		node.Path = ""
		return nil
	case *WyWebPost, *WyWebListing, *WyWebGallery:
		if !node.Parent.resolved {
			node.Parent.resolve()
		}
		copyHeadData(&node.HeadData, (*meta).GetHeadData())
		mergePageData(&node.PageData, (*meta).GetPageData())
		if node.Meta == nil {
			node.Meta = make([]string, 0)
		}
		node.Meta = append(node.Meta, node.Parent.Meta...)
		node.resolveIncludes()
	default:
		log.Printf("Meta: %+v\n", *meta)
		log.Printf("%+v", node.PageData)
	}
	switch t := (*meta).(type) {
	case *WyWebPost:
		if node.Index == "" {
			node.Index = t.Index
		}
		_, err := os.Stat(node.Index)
		if err != nil {
			_, err = os.Stat(filepath.Join(node.Path, node.Index))
			if err == nil {
				node.Index = filepath.Join(node.Path, node.Index)
			} else {
				log.Printf("WARN: Could not find index for %s specified at %s", node.Path, node.Index)
				return fmt.Errorf("could not find index for %s specified at %s", node.Path, node.Index)
			}
		}
		if node.Tags == nil {
			node.Tags = make([]string, len(t.Tags))
			copy(node.Tags, t.Tags)
		} else {
			node.Tags = util.ConcatUnique(node.Tags, t.Tags)
			if node.Preview == "" {
				node.Preview = t.Preview
			}
		}
	case *WyWebGallery:
		node.Images = make([]RichImage, len(t.GalleryItems))
		copy(node.Images, t.GalleryItems)
		for idx := range node.Images {
			node.Images[idx].ParentPage = node
		}
	}
	return nil
}

func (node *ConfigNode) registerTags() {
	tree := node.Tree
	switch node.NodeKind {
	case WWPOST:
		for _, tag := range node.Tags {
			regiserTag(tag, node, &node.Tree.TagDB)
			if node.Parent != tree.Root {
				regiserTag(tag, node, &node.Parent.TagDB)
			}
		}
	case WWGALLERY:
		for _, item := range node.Images {
			for _, tag := range item.Tags {
				regiserTag(tag, &item, &node.Tree.TagDB)
			}
		}
	}
}

func (node *ConfigNode) setAbsoluteIndex() error {
	_, err := os.Stat(node.Index)
	if err == nil {
		return nil
	}
	idx := filepath.Join(node.RealPath, node.Index)
	_, err = os.Stat(idx)
	if err == nil {
		node.Index = idx
		return nil
	}
	idx = filepath.Join(filepath.Dir(node.RealPath), node.Index)
	_, err = os.Stat(idx)
	if err == nil {
		node.Index = idx
		return nil
	}
	return fmt.Errorf("could not find absolute path for index of %s (%s)", node.Title, node.Path)
}

func (node *ConfigNode) Magic() {
	if node.Title == "" {
		switch node.NodeKind {
		case WWPOST:
			GetTitleFromMarkdown(node, nil, nil)
		}
	}
}

func (node *ConfigNode) resolve() error {
	//node.Lock()
	//defer node.Unlock()
	if node.resolved {
		return nil
	}
	if node.Data != nil {
		err := node.SetFieldsFromWyWebMeta(node.Data)
		if err != nil {
			return err
		}
	}
	err := node.setAbsoluteIndex()
	if err != nil {
		log.Println(err.Error())
		return err
	}
	node.Magic()
	if node.StructuredData == nil {
		node.StructuredData = make([]string, 0)
	}
	if node.RealPath == "" {
		node.RealPath = node.Path
	}
	node.SetID()
	node.registerTags()
	node.resolved = true
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
	//node.Lock()
	//defer node.Unlock()
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
	for i := range siblings {
		if i > 0 {
			path = "/" + siblings[i-1].Path
			text = siblings[i-1].Title
			setNavLink(&siblings[i].Prev, path, text)
		}
		path, _ = filepath.Rel(".", node.Path)
		path = "/" + path
		text = node.Title
		setNavLink(&siblings[i].Up, path, text)
		if i < len(siblings)-1 {
			path = "/" + siblings[i+1].Path
			text = siblings[i+1].Title
			setNavLink(&siblings[i].Next, path, text)
		}
	}
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
	ignore := []string{".git"}
	for _, file := range files {
		if slices.Contains(ignore, file.Name()) {
			continue
		}
		var meta WyWebMeta
		var e error
		var path, key string
		var child *ConfigNode
		if !file.IsDir() {
			if strings.HasSuffix(file.Name(), ".post.md") {
				child = MagicPost(node, file.Name())
				key = strings.TrimSuffix(file.Name(), ".post.md")
				path = filepath.Join(dir, file.Name())
			} else {
				continue
			}
		} else {
			path = filepath.Join(dir, file.Name())
			key = path
			wwFileName := filepath.Join(dir, file.Name(), "wyweb")
			_, e = os.Stat(wwFileName)
			if e != nil {
				if strings.HasSuffix(file.Name(), ".listing") || strings.ToLower(file.Name()) == "blog" {
					key = filepath.Join(dir, strings.TrimSuffix(file.Name(), ".listing"))
					path = filepath.Join(dir, file.Name())
					child = MagicListing(node, file.Name())
				} else {
					continue
				}
			} else {
				meta, e = ReadWyWeb(path)
				if e != nil {
					log.Printf("couldn't read %s", path)
					continue
				}
				child = &ConfigNode{
					Parent:   node,
					Data:     &meta,
					Children: make(map[string]*ConfigNode),
					TagDB:    make(map[string][]Listable),
					Tree:     tree,
				}
				(*child).Path = path
			}
		}
		err := child.resolve()
		if err != nil {
			log.Println(err.Error())
			continue
		}
		node.Children[filepath.Base(key)] = child
		child.growTree(path, tree)
	}
	setNavLinksOfChildren(node)
	return status
}

func BuildConfigTree(documentRoot string, domain string) (*ConfigTree, error) {
	var err error
	rootnode := ConfigNode{
		Parent:   nil,
		Data:     nil,
		Children: make(map[string]*ConfigNode),
		Tree:     nil,
		TagDB:    make(map[string][]Listable),
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

func (node *ConfigNode) GetHTMLHeadData() *HTMLHeadData {
	//node.RLock()
	//defer node.RUnlock()
	styles := make([]interface{}, 0)
	scripts := make([]interface{}, 0)
	for _, name := range node.LocalResources {
		res, ok := node.Tree.Resources[name]
		if !ok {
			log.Printf("%s does not exist in the resource registry.\n", name)
			continue
		}
		if res.Value == "" {
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
		Title:   node.Title,
		Meta:    node.Meta,
		Styles:  styles,
		Scripts: scripts,
	}
	return out
}
