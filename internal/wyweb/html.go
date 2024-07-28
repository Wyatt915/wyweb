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
	"bytes"
	"fmt"
	"slices"
	"strings"
)

var voidElements = []string{
	"area",
	"base",
	"br",
	"col",
	"embed",
	"hr",
	"img",
	"input",
	"link",
	"meta",
	"param", //Deprecated
	"source",
	"track",
	"wbr",
}

type HTMLElement struct {
	Tag        string
	Content    string
	Attributes map[string]string
	Children   []*HTMLElement
	indent     bool
}

func NewHTMLElement(tag string, attr ...map[string]string) *HTMLElement {
	attributes := make(map[string]string)
	for _, attributeList := range attr {
		if attributeList == nil {
			continue
		}
		for key, value := range attributeList {
			_, ok := attributes[key]
			if !ok {
				attributes[key] = value
			} else {
				attributes[key] = attributes[key] + " " + value
			}
		}
	}
	return &HTMLElement{
		Tag:        tag,
		Content:    "",
		Attributes: attributes,
		Children:   make([]*HTMLElement, 0),
		indent:     true,
	}
}

func (e *HTMLElement) NoIndent() {
	e.indent = false
}

func (e *HTMLElement) Append(elem *HTMLElement) {
	e.Children = append(e.Children, elem)
}

// Convienience function to quickly make a class attribute
func Class(cls string) map[string]string {
	return map[string]string{"class": cls}
}

// Convienience function to quickly make a class attribute
func ID(id string) map[string]string {
	return map[string]string{"id": id}
}

func AriaLabel(lbl string) map[string]string {
	return map[string]string{"aria-label": lbl}
}

// Convienience function to quickly make an href attribute
func Href(url string) map[string]string {
	return map[string]string{"href": url}
}

func (e *HTMLElement) AppendNew(tag string, attr ...map[string]string) *HTMLElement {
	elem := NewHTMLElement(tag)
	e.Children = append(e.Children, elem)
	for _, attributeList := range attr {
		if attributeList == nil {
			continue
		}
		for key, value := range attributeList {
			elem.Attributes[key] = value
		}
	}
	return elem
}

func (e *HTMLElement) AppendText(text string) *HTMLElement {
	elem := &HTMLElement{
		Tag:        "",
		Content:    text,
		Attributes: nil,
		Children:   nil,
		indent:     true,
	}
	e.Children = append(e.Children, elem)
	return elem
}

func isShort(elem *HTMLElement) (bool, int) {
	if elem == nil {
		return false, 0
	}
	if len(elem.Children) > 1 {
		return false, 0
	}
	if len(elem.Children) == 0 {
		if elem.Tag == "" {
			return true, len(elem.Content)
		}
		return true, 0
	}
	if elem.Children[0].Tag != "" {
		return false, 0
	}
	return true, len(elem.Children[0].Content)
}

func openTag(elem *HTMLElement, depth int) []byte {
	var out bytes.Buffer
	for i := 0; i < depth; i++ {
		out.WriteString("    ")
	}
	out.WriteByte('<')
	out.WriteString(elem.Tag)
	for key, value := range elem.Attributes {
		out.WriteByte(' ')
		out.WriteString(key)
		if value != "" {
			out.WriteByte('=')
			out.WriteByte('"')
			out.WriteString(value)
			out.WriteByte('"')
		}
	}
	if slices.Contains(voidElements, elem.Tag) {
		out.WriteString(">\n")
	} else if short, textlen := isShort(elem); short && textlen < 32 {
		out.WriteByte('>')
	} else {
		out.WriteString(">\n")
	}
	return out.Bytes()
}

func closeTag(elem *HTMLElement, depth int) []byte {
	var out bytes.Buffer
	if short, textlen := isShort(elem); !(short && textlen < 32) {
		for i := 0; i < depth; i++ {
			out.WriteString("    ")
		}
	}
	out.WriteString("</")
	out.WriteString(elem.Tag)
	out.WriteString(">\n")
	return out.Bytes()
}

func RenderHTML(root *HTMLElement, text *bytes.Buffer, opts ...int) {
	if root == nil {
		return
	}
	var depth int
	var siblings int
	if len(opts) > 0 {
		depth = opts[0]
	}
	if len(opts) > 1 {
		siblings = opts[1]
	}
	if root.Tag == "" {
		lines := strings.Split(root.Content, "\n")
		for _, line := range lines {
			if short, textlen := isShort(root); !short || textlen >= 32 || len(lines) > 1 || siblings > 1 {
				if root.indent {
					for i := 0; i < depth; i++ {
						text.WriteString("    ")
					}
				}
				text.WriteString(line)
				text.WriteByte('\n')
			} else {
				text.WriteString(strings.TrimSpace(line))
			}
		}
		return
	}
	text.Write(openTag(root, depth))
	for _, elem := range root.Children {
		RenderHTML(elem, text, depth+1, len(root.Children))
	}
	// void elements should not have a closing tag!
	if !slices.Contains(voidElements, root.Tag) {
		text.Write(closeTag(root, depth))
	}
}

func (e *HTMLElement) GetElementByID(id string) (*HTMLElement, error) {
	if e == nil {
		return nil, fmt.Errorf("cannot search nil HTMLElement")
	}
	if thisID, ok := e.Attributes["id"]; ok {
		idList := strings.Split(thisID, " ")
		if slices.Contains(idList, id) {
			return e, nil
		}
	}
	for _, child := range e.Children {
		elem, err := child.GetElementByID(id)
		if err == nil {
			return elem, err
		}
	}
	return nil, fmt.Errorf("no element found matching #%s", id)
}

func (e *HTMLElement) FirstElementByClass(classes ...string) (*HTMLElement, error) {
	if e == nil {
		return nil, fmt.Errorf("cannot search nil HTMLElement")
	}
	if classListStr, ok := e.Attributes["class"]; ok {
		CurrentClassList := strings.Split(classListStr, " ")
		allClassesMatch := true
		for _, cls := range classes {
			allClassesMatch = allClassesMatch && slices.Contains(CurrentClassList, cls)
		}
		if allClassesMatch {
			return e, nil
		}
	}
	for _, child := range e.Children {
		elem, err := child.FirstElementByClass(classes...)
		if err == nil {
			return elem, err
		}
	}
	return nil, fmt.Errorf("no element found matching .%s", strings.Join(classes, "."))
}

func (e *HTMLElement) RemoveNode(target *HTMLElement) bool {
	//removeIndex := -1
	for idx, child := range e.Children {
		if child == target {
			//removeIndex = idx
			//e.Children[idx] = nil
			e.Children = append(e.Children[:idx], e.Children[idx+1:]...)
			return true
		}
		if child.RemoveNode(target) {
			return true
		}
	}
	//if removeIndex >= 0 {
	//	e.Children = append(e.Children[:removeIndex], e.Children[removeIndex+1:]...)
	//	return true
	//}
	return false
}

func printHTML(elem *HTMLElement) {
	var buf bytes.Buffer
	RenderHTML(elem, &buf)
	fmt.Printf("%s\n", buf.String())
}
