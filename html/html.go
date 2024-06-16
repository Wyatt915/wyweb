package html

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
}

func NewHTMLElement(tag string) *HTMLElement {
	return &HTMLElement{
		Tag:        tag,
		Content:    "",
		Attributes: make(map[string]string),
		Children:   make([]*HTMLElement, 0),
	}
}

func (e *HTMLElement) Append(elem *HTMLElement) {
	e.Children = append(e.Children, elem)
}

// Convienience function to quickly make a class attribute
func Class(cls string) map[string]string {
	return map[string]string{"class": cls}
}

// Convienience function to quickly make an href attribute
func Href(url string) map[string]string {
	return map[string]string{"href": url}
}

func (e *HTMLElement) AppendNew(tag string, attr ...map[string]string) *HTMLElement {
	elem := NewHTMLElement(tag)
	fmt.Printf("Raw attr: %v\n\n", attr)
	e.Children = append(e.Children, elem)
	for _, attributeList := range attr {
		if attributeList == nil {
			continue
		}
		fmt.Printf("%v\n", attributeList)
		for key, value := range attributeList {
			elem.Attributes[key] = value
		}
	}
	fmt.Printf("Full attribute list: %v\n", elem.Attributes)
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
	} else if len(elem.Children) == 0 && len(elem.Content) < 32 {
		out.WriteByte('>')
	} else {
		out.WriteString(">\n")
	}
	return out.Bytes()
}

func closeTag(elem *HTMLElement, depth int) []byte {
	var out bytes.Buffer
	if len(elem.Children) != 0 || len(elem.Content) >= 32 {
		for i := 0; i < depth; i++ {
			out.WriteString("    ")
		}
	}
	out.WriteString("</")
	out.WriteString(elem.Tag)
	out.WriteString(">\n")
	return out.Bytes()
}

func RenderHTML(root *HTMLElement, text *bytes.Buffer, optDepth ...int) {
	var depth int
	if len(optDepth) > 0 {
		depth = optDepth[0]
	}
	if root.Tag == "" {
		lines := strings.Split(root.Content, "\n")
		for _, line := range lines {
			for i := 0; i < depth; i++ {
				text.WriteString("    ")
			}
			text.WriteString(strings.TrimSpace(line))
			text.WriteByte('\n')
		}
		return
	}
	text.Write(openTag(root, depth))
	for _, elem := range root.Children {
		RenderHTML(elem, text, depth+1)
	}
	// void elements should not have a closing tag!
	if !slices.Contains(voidElements, root.Tag) {
		text.Write(closeTag(root, depth))
	}
}
