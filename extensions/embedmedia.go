package extensions

import (
	"bytes"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type mediaType int

const (
	mediaAudio = iota
	mediaVideo
)

type mediaInfo struct {
	ext         string
	destination []byte
}

type media struct {
	ast.BaseInline
	info   mediaInfo
	medium mediaType
}

var KindMedia = ast.NewNodeKind("Media")

func (n *media) Kind() ast.NodeKind {
	return KindMedia
}

// Dump implements Node.Dump.
func (n *media) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func NewMedia(i mediaInfo, t mediaType) *media {
	return &media{
		info:   i,
		medium: t,
	}
}

// var contextKeySnippet = parser.NewContextKey()
type mediaTransformer struct{}

func (r mediaTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	//var buf bytes.Buffer
	VideoExt := []string{"webm", "mp4", "mkv", "ogv"}
	AudioExt := []string{"mp3", "ogg", "wav", "flac"}
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindImage {
			//var imagenode bytes.Buffer
			img := n.(*ast.Image)
			dotidx := bytes.LastIndexByte(img.Destination, '.')
			if dotidx >= 0 {
				ext := string(img.Destination[dotidx+1:])
				isMedia := false
				var flavor mediaType
				if slices.Contains(VideoExt, ext) {
					flavor = mediaVideo
					isMedia = true
				} else if slices.Contains(AudioExt, ext) {
					flavor = mediaAudio
					isMedia = true
				}
				if isMedia {
					n.Parent().ReplaceChild(n.Parent(), n, NewMedia(mediaInfo{ext, img.Destination}, flavor))
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

// Create Renderer
// MediaHTMLRenderer is a renderer for video nodes.
type MediaHTMLRenderer struct{}

// NewMediaHTMLRenderer returns a new MediaHTMLRenderer.
func NewMediaHTMLRenderer() renderer.NodeRenderer {
	return &MediaHTMLRenderer{}
}

// RegisterFuncs registers the renderer with the Goldmark renderer.
func (r *MediaHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindMedia, r.renderMedia)
}

func (r *MediaHTMLRenderer) renderMedia(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*media)
	if !ok {
		return ast.WalkContinue, nil
	}

	var tagOpen string
	var tagClose string
	var mime string

	switch n.medium {
	case mediaVideo:
		tagOpen = `<video controls autoplay loop mute>`
		tagClose = `</video>`
		mime = "video"
	case mediaAudio:
		tagOpen = `<audio controls>`
		tagClose = `</audio>`
		mime = "audio"
	}

	mime = strings.Join([]string{mime, n.info.ext}, "/")
	sourceTag := []string{
		`<source src="`,
		string(n.info.destination),
		`" type="`,
		mime,
		`" />`,
	}

	if entering {
		_, _ = w.WriteString(tagOpen)
		_, _ = w.WriteString(strings.Join(sourceTag, ""))
		_, _ = w.WriteString(tagClose)
	}

	return ast.WalkContinue, nil
}

type mediaEmbed struct{}

func (e *mediaEmbed) Extend(m goldmark.Markdown) {
	p := int(^uint(0) >> 1) // Lowest priority
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(mediaTransformer{}, 9000),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(NewMediaHTMLRenderer(), p),
		),
	)
}

func EmbedMedia() goldmark.Extender {
	return &mediaEmbed{}
}
