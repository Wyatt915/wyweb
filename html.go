package main

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
	depth      int
}

func NewHTMLElement(tag string) *HTMLElement {
	return &HTMLElement{
		Tag:        tag,
		Content:    "",
		Attributes: make(map[string]string),
		Children:   make([]*HTMLElement, 0),
		depth:      0,
	}
}

func (e *HTMLElement) increaseDepth(amt int) {
	e.depth += amt
	for _, child := range e.Children {
		child.increaseDepth(amt)
	}
}

func (e *HTMLElement) Append(elem *HTMLElement) {
	e.Children = append(e.Children, elem)
	depthDiff := e.depth - elem.depth
	elem.increaseDepth(depthDiff + 1)
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
	elem.depth = e.depth + 1
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
	elem.depth = e.depth + 1
	return elem
}

func openTag(elem *HTMLElement) []byte {
	var out bytes.Buffer
	for i := 0; i < elem.depth; i++ {
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

func closeTag(elem *HTMLElement) []byte {
	var out bytes.Buffer
	if len(elem.Children) != 0 || len(elem.Content) >= 32 {
		for i := 0; i < elem.depth; i++ {
			out.WriteString("    ")
		}
	}
	out.WriteString("</")
	out.WriteString(elem.Tag)
	out.WriteString(">\n")
	return out.Bytes()
}

func RenderHTML(root *HTMLElement, text *bytes.Buffer) {
	if root.Tag == "" {
		lines := strings.Split(root.Content, "\n")
		for _, line := range lines {
			for i := 0; i < root.depth; i++ {
				text.WriteString("    ")
			}
			text.WriteString(strings.TrimSpace(line))
			text.WriteByte('\n')
		}
		return
	}
	text.Write(openTag(root))
	for _, elem := range root.Children {
		RenderHTML(elem, text)
	}
	// void elements should not have a closing tag!
	if !slices.Contains(voidElements, root.Tag) {
		text.Write(closeTag(root))
	}
}
