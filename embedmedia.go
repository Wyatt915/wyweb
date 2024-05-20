package main

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type MediaType int

const (
	MediaAudio = iota
	MediaVideo
)

type MediaInfo struct {
	ext         string
	Destination []byte
}

type Media struct {
	ast.BaseInline
	info   MediaInfo
	medium MediaType
}

var KindMedia = ast.NewNodeKind("Media")

func (n *Media) Kind() ast.NodeKind {
	return KindMedia
}

// Dump implements Node.Dump.
func (n *Media) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func NewMedia(i MediaInfo, t MediaType) *Media {
	return &Media{
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
				if slices.Contains(VideoExt, ext) {
					n.Parent().ReplaceChild(n.Parent(), n, NewMedia(MediaInfo{ext, img.Destination}, MediaVideo))
				} else if slices.Contains(AudioExt, ext) {
					n.Parent().ReplaceChild(n.Parent(), n, NewMedia(MediaInfo{ext, img.Destination}, MediaAudio))
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

// Create Renderer
// VideoHTMLRenderer is a renderer for video nodes.
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
	n, ok := node.(*Media)
	if !ok {
		return ast.WalkContinue, nil
	}

	var tagOpen string
	var tagClose string
	var mime string

	switch n.medium {
	case MediaVideo:
		tagOpen = `<video controls>`
		tagClose = `<\video>`
		mime = "video"
	case MediaAudio:
		tagOpen = `<audio controls>`
		tagClose = `<\audio>`
		mime = "audio"
	}

	mime = strings.Join([]string{mime, n.info.ext}, "/")
	sourceTag := []string{
		`<source src="`,
		string(n.info.Destination),
		`" type="`,
		mime,
		`" />`,
	}

	if entering {
		_, _ = w.WriteString(tagOpen)
		_, _ = w.WriteString(strings.Join(sourceTag, " "))
		_, _ = w.WriteString(tagClose)
	}

	return ast.WalkContinue, nil
}

type mediaExtension struct{}

func (e *mediaExtension) Extend(m goldmark.Markdown) {
	p := int(^uint(0) >> 1) // Lowest priority
	fmt.Println(p)
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(mediaTransformer{}, p),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(NewMediaHTMLRenderer(), p),
		),
	)
}

func MediaExtension() goldmark.Extender {
	return &mediaExtension{}
}
