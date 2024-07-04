package extensions

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
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
	kv := map[string]string{"flag": n.flag}
	ast.DumpHelper(n, source, level, kv, nil)
}

func NewAlertFlag(f string) *alertFlagNode {
	return &alertFlagNode{flag: strings.ToLower(f)}
}

func (p *alertParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	var (
		_open  = []byte("[!")
		_close = []byte("]")
	)
	//Not in a blockquote? Ignore it!
	if _, ok := parent.Parent().(*ast.Blockquote); !ok {
		return nil
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
	alertName = strings.TrimFunc(alertName, unicode.IsSpace)
	if strings.ContainsFunc(alertName, unicode.IsSpace) {
		return nil
	}
	if strings.ContainsFunc(alertName, unicode.IsLower) {
		return nil
	}
	out := NewAlertFlag(alertName)
	out.AppendChild(out, ast.NewTextSegment(text.NewSegment(seg.Start, seg.Start+stop+len(_close))))
	block.Advance(stop + 1)
	return out
}

type alertNode struct {
	ast.BaseBlock
	alertName string
}

var KindAlert = ast.NewNodeKind("Alert")

func (n *alertNode) Kind() ast.NodeKind {
	return KindAlert
}

// Dump implements Node.Dump.
func (n *alertNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// capitalize the first letter of the string
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	s = strings.ToLower(s)
	return string(unicode.ToTitle(r)) + s[size:]
}

func NewAlert(name string) *alertNode {
	return &alertNode{alertName: strings.ToLower(name)}
}

type alertTransformer struct{}

func (t alertTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	continueTransform := true
	for continueTransform {
		continueTransform = false
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if bq, ok := n.(*ast.Blockquote); ok && entering {
				parent := bq.Parent()
				if para, okP := bq.FirstChild().(*ast.Paragraph); okP {
					if flag, okF := para.FirstChild().(*alertFlagNode); okF {
						continueTransform = true
						alert := NewAlert(flag.flag)
						class := "alert alert-" + flag.flag
						alert.SetAttribute([]byte("class"), class)
						alert.SetLines(bq.Lines())
						alert.SetBlankPreviousLines(bq.HasBlankPreviousLines())
						para.RemoveChild(para, flag)
						child := bq.FirstChild()
						for child != nil {
							next := child.NextSibling()
							alert.AppendChild(alert, child)
							child = next
						}
						parent.ReplaceChild(parent, bq, alert)
						return ast.WalkStop, nil
					}
				}
			}
			return ast.WalkContinue, nil
		})
	}
}

type AlertRenderer struct{}

// NewAlertRenderer returns a new AlertRenderer.
func NewAlertRenderer() renderer.NodeRenderer {
	return &AlertRenderer{}
}

// RegisterFuncs registers the renderer with the Goldmark renderer.
func (r *AlertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindAlert, r.renderAlert)
}

func (r *AlertRenderer) renderAlert(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*alertNode)
	if !ok {
		return ast.WalkContinue, nil
	}
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<div")
			html.RenderAttributes(w, n, html.GlobalAttributeFilter)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<div>\n")
		}
		_, _ = w.WriteString(`<p class="alert-title alert-`)
		_, _ = w.WriteString(n.alertName)
		_, _ = w.WriteString(`">`)
		_, _ = w.WriteString(capitalize(n.alertName))
		_, _ = w.WriteString("</p>\n")
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}

type alertExtension struct{}

func (e *alertExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(newAlertParser(), priorityAlertParser),
		),
		parser.WithASTTransformers(
			util.Prioritized(alertTransformer{}, priorityAlertTransformer),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(NewAlertRenderer(), priorityAlertRenderer),
		),
	)
}

func AlertExtension() goldmark.Extender {
	return &alertExtension{}
}
