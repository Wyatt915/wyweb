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

package extensions

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type attribListParser struct{}

func NewAttribListParser() *attribListParser {
	return &attribListParser{}
}

var (
	_open  = []byte("{:")
	_close = []byte("}")
)

func (p *attribListParser) Trigger() []byte {
	return []byte{'{'}
}

type attrNode struct {
	ast.BaseInline
}

var KindAttrList = ast.NewNodeKind("AttrList")

func (n *attrNode) Kind() ast.NodeKind {
	return KindAttrList
}
func (n *attrNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func parseAttrList(attrstr []byte) attrNode {
	//classSelector := '.'
	//idSelector := '#'
	//result := make([]ast.Attribute, 0)
	result := attrNode{}
	i := 0
	classes := make([]string, 0)
	ids := make([]string, 0)
	for i < len(attrstr) {
		var value strings.Builder
		var name string
		keepGoing := true
	Loop:
		for j := i; j < len(attrstr) && keepGoing; j++ {
			switch attrstr[j] {
			case ' ', '\t':
				i = j + 1
				keepGoing = false
				break Loop
			case '.':
				name = "class"
				i++
			case '#':
				name = "id"
				i++
			default:
				value.WriteByte(attrstr[j])
				i++
			}
		}
		if value.Len() == 0 {
			continue
		}
		switch name {
		case "class":
			classes = append(classes, value.String())
		case "id":
			ids = append(ids, value.String())
		}
		value.Reset()
		name = ""
	}
	if len(classes) > 0 {
		result.SetAttribute([]byte("class"), strings.Join(classes, " "))
	}
	if len(ids) > 0 {
		result.SetAttribute([]byte("id"), strings.Join(ids, " "))
	}
	return result
}

func (p *attribListParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	stop := bytes.Index(line, _close)
	if stop < 0 {
		return nil
	}
	if !bytes.HasPrefix(line, _open) {
		return nil
	}
	seg = text.NewSegment(len(_open), stop)
	block.Advance(stop + 1)
	resNode := parseAttrList(line[seg.Start:seg.Stop])
	return &resNode
}

type attribListTransformer struct{}

func (r attribListTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && (n.Kind() == KindAttrList) {
			sib := n.PreviousSibling()
			if sib == nil {
				n.Parent().RemoveChild(n.Parent(), n)
				return ast.WalkContinue, nil
			}
			for _, attr := range n.Attributes() {
				sib.SetAttribute(attr.Name, attr.Value)
			}
		}
		return ast.WalkContinue, nil
	})
}

type attribList struct{}

func (e *attribList) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(NewAttribListParser(), priorityAttribListParser),
		),
		parser.WithASTTransformers(
			util.Prioritized(attribListTransformer{}, priorityAttribListTransformer),
		),
	)
}

func AttributeList() goldmark.Extender {
	return &attribList{}
}
