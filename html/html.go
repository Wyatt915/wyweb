package html

import (
	"bytes"
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
			attributes[key] = value
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
			if short, textlen := isShort(root); root.indent && (!short || textlen >= 32 || len(lines) > 1 || siblings > 1) {
				for i := 0; i < depth; i++ {
					text.WriteString("    ")
				}
				text.WriteString(strings.TrimSpace(line))
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
