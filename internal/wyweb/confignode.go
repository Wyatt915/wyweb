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
	"encoding/base64"
	"encoding/binary"
	"html"
	"log"
	"math/bits"
	"os"
	"sync"
	"time"

	"github.com/yuin/goldmark/ast"
)

type DependencyKind int

const (
	KindMDSource = iota
	KindResourceLocal
	KindWyWeb
	KindFileEmbed
)

type ConfigNode struct {
	sync.RWMutex
	PageData
	HeadData
	// An id is constructed from the Date, Updated, and Resolved.Title fields.
	// The first 16 bits count the number of days after 2000-01-01 the item was published.
	// The next 8 bits count the number of days since the last update at the time the id was computed.
	// The final 40 bits are the concatenation of UTF-8 codepoints (with leading zeroes removed) taken from
	// Resolved.Title.
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
	Dependencies   map[string]DependencyKind //All files on which this node depends
	knownFiles     []string
	LastRead       time.Time
}

type Listable interface {
	GetDate() time.Time
	GetID() uint64
	GetTitle() string
	SetID()
	AsRSSItem() *HTMLElement
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

func (n *ConfigNode) AsRSSItem() *HTMLElement {
	item := NewHTMLElement("item")
	item.AppendNew("title").AppendText(n.Title)
	item.AppendNew("link").AppendText("https://" + n.Tree.Domain + "/" + n.Path)
	item.AppendNew("description").AppendText(html.EscapeString(n.Description))
	item.AppendNew("pubDate").AppendText(n.Date.Format(time.RFC1123Z))
	return item
}

func (n *ConfigNode) GetIDb64() string {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n.id)
	return base64.URLEncoding.EncodeToString(bs)
}

func newConfigNode() *ConfigNode {
	var out ConfigNode
	out.Children = make(map[string]*ConfigNode)
	out.TagDB = make(map[string][]Listable)
	out.Dependencies = make(map[string]DependencyKind)
	return &out
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
		case "local":
			temp, err := os.ReadFile(res.Value)
			if err == nil {
				value = RawResource{String: string(temp), Attributes: res.Attributes}
			}
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
