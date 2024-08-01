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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"wyweb.site/util"
)

// includeExclude resolves which includables (styles or scripts) will be used on the page.
// local - the new includables of the cueent node
// include - the in
func includeExclude(local []string, include []string, exclude []string) []string {
	result := make([]string, 0)
	for _, name := range local {
		if slices.Contains(exclude, name) {
			log.Printf("WARN: The locally defined %s, is already defined and set to be excluded. The new definition will be ignored.\n", name)
			continue
		} else if slices.Contains(include, name) {
			log.Printf("WARN: The locally defined %s, is already defined. The new definition will be ignored.\n", name)
		}
		result = append(result, name)
	}
	for _, name := range include {
		//exists := slices.Contains(local, name)
		excluded := slices.Contains(exclude, name)
		if !excluded {
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
	if node.Tree.Root == node {
		return
	}
	node.LocalResources = make([]string, 0)
	local := make([]string, 0)
	for name, value := range node.Resources {
		if value.Method == "url" || value.Method == "local" {
			var err error
			value.Value, err = util.RewriteURLPath(value.Value, node.RealPath)
			if err != nil {
				log.Println(err.Error())
				continue
			}
		}
		if value.Method == "local" {
			value.Value = strings.TrimLeft(value.Value, string(os.PathSeparator))
			node.Dependencies[value.Value] = KindResourceLocal
		}
		local = append(local, name)
		_, ok := node.Tree.Resources[name]
		if ok {
			log.Printf("WARN: In configuration %s, the Resource %s is already defined. The new definition will be ignored.\n", node.Path, name)
		} else {
			node.Tree.Resources[name] = value
		}
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
	node.NodeKind = (*meta).GetType()
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
	switch node.NodeKind {
	case WWPOST:
		mdfile, err := os.ReadFile(node.Index)
		if node.Title == "" {
			if err == nil {
				GetTitleFromMarkdown(node, mdfile, nil)
			}
		}
		if node.Preview == "" {
			if err == nil {
				GetPreviewFromMarkdown(node, mdfile, nil)
			}
		}
		if node.Description == "" {
			node.Description = node.Preview
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
	node.Dependencies[node.Index] = KindMDSource
	node.Magic()
	if node.StructuredData == nil {
		node.StructuredData = make([]string, 0)
	}
	if node.RealPath == "" {
		node.RealPath = node.Path
	}
	node.resolveIncludes()
	node.inheritIfUndefined()
	node.SetID()
	node.registerTags()
	node.LastRead = time.Now()
	node.resolved = true
	//fmt.Printf("%s\n\t", node.Title)
	//for k, v := range node.Dependencies {
	//	fmt.Printf("%s (%d)\t", k, v)
	//}
	//fmt.Println()
	return nil
}

func setNavLink(nl *WWNavLink, path, title string) {
	//TODO: See if these fields were preset in the wyweb file
	//if nl.Path == "" {
	//	nl.Path = path
	//}
	//if nl.Text == "" {
	//	nl.Text = title
	//}
	nl.Path = path
	nl.Text = title
}

func rerenderNavLinks(node *ConfigNode) {
	oldnav, err := node.HTML.FirstElementByClass("navlinks")
	if err != nil {
		return
	}
	*oldnav = *BuildNavlinks(node)
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
		} else {
			setNavLink(&siblings[i].Prev, "", "")
		}
		path, _ = filepath.Rel(".", node.Path)
		path = "/" + path
		text = node.Title
		setNavLink(&siblings[i].Up, path, text)
		if i < len(siblings)-1 {
			path = "/" + siblings[i+1].Path
			text = siblings[i+1].Title
			setNavLink(&siblings[i].Next, path, text)
		} else {
			setNavLink(&siblings[i].Next, "", "")
		}
	}
	for _, sib := range siblings {
		rerenderNavLinks(sib)
	}
}
