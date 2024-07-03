package extensions

import (
	"bytes"
	"log"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type alertParser struct{}

func (p *alertParser) Trigger() []byte {
	return []byte{'['}
}

func newAlertParser() *alertParser {
	return &alertParser{}
}

type alertFlagNode struct {
	ast.BaseInline
	flag string
}

var KindAlertFlag = ast.NewNodeKind("AlertFlag")

func (n *alertFlagNode) Kind() ast.NodeKind {
	return KindAlertFlag
}

// Dump implements Node.Dump.
func (n *alertFlagNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func NewAlertFlag(f string) *alertFlagNode {
	return &alertFlagNode{
		flag: f,
	}
}

func (p *alertParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	var (
		_open  = []byte("[!")
		_close = []byte("]")
	)
	if grandparent, ok := parent.Parent().(*ast.Blockquote); ok {
		grandparent.Dump(block.Source(), 0)
	}
	line, seg := block.PeekLine()
	stop := bytes.Index(line, _close)
	if stop < 0 {
		return nil
	}
	if !bytes.HasPrefix(line, _open) {
		return nil
	}
	alertName := string(block.Value(text.NewSegment(seg.Start+len(_open), seg.Start+stop)))
	out := NewAlertFlag(alertName)
	out.AppendChild(out, ast.NewTextSegment(text.NewSegment(seg.Start, seg.Start+stop+len(_close))))
	block.Advance(stop + 1)
	return out
}

type alert int

type alertTransformer struct{}

func (t alertTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if bq, ok := n.(*ast.Blockquote); ok && entering {
			if para, ok := bq.FirstChild().(*ast.Paragraph); ok {
				if flag, ok := para.FirstChild().(*alertFlagNode); ok {
					var class string
					switch flag.flag {
					case "NOTE":
						class = "alert alert-note"
					case "TIP":
						class = "alert alert-tip"
					case "IMPORTANT":
						class = "alert alert-important"
					case "WARNING":
						class = "alert alert-warning"
					case "CAUTION":
						class = "alert alert-caution"
					default:
						return ast.WalkContinue, nil
					}
					classAttr, ok := bq.AttributeString("class")
					if !ok {
						bq.SetAttribute([]byte("class"), class)
					} else {
						switch t := classAttr.(type) {
						case string:
							bq.SetAttribute([]byte("class"), strings.Join([]string{t, class}, " "))
						default:
							log.Println("Unknown type of class attribute for alert:", flag.flag)
						}
					}
					para.RemoveChild(para, flag)
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

type alertExtension struct{}

func (e *alertExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(alertTransformer{}, priorityAlertTransformer),
		),
		parser.WithInlineParsers(
			util.Prioritized(newAlertParser(), priorityAlertParser),
		),
	)
}

func AlertExtension() goldmark.Extender {
	return &alertExtension{}
}
