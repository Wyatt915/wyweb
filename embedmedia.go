package main

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Create new Kinds: Media, Audio, and Video. Audio and Video. Media is the base type embedded by both Audio and Video.

type Media struct {
	ast.BaseInline
	URLset [][]byte
}

var KindMedia = ast.NewNodeKind("Media")

func (n *Media) Kind() ast.NodeKind {
	return KindMedia
}

type Video struct {
	Media
}

type Audio struct {
	Media
}

// Dump implements Node.Dump.
func (n *Media) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

var KindVideo = ast.NewNodeKind("Video")
var KindAudio = ast.NewNodeKind("Audio")

func (n *Video) Kind() ast.NodeKind {
	return KindVideo
}
func NewVideo(url [][]byte) *Video {
	return &Video{
		Media: Media{URLset: url},
	}
}

func (n *Audio) Kind() ast.NodeKind {
	return KindAudio
}
func NewAudio(url [][]byte) *Audio {
	return &Audio{
		Media: Media{URLset: url},
	}
}

// Create Parser

type mediaParser struct{}

// NewMediaParser returns a new mediaParser.
func NewMediaParser() parser.InlineParser {
	return &mediaParser{}
}

// Trigger returns characters that trigger this parser.
func (p *mediaParser) Trigger() []byte {
	return []byte{'!', '['}
}

// Parse parses a media element.
func (p *mediaParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	pattern := regexp.MustCompile(`!\[(.*)\]\((.*)\)`)

	indecies := pattern.FindAllSubmatchIndex(line, -1)
	matches := pattern.FindAllSubmatch(line, -1)

	VideoExt := []string{".webm", ".mp4", ".mkv", ".ogv"}
	AudioExt := []string{".mp3", ".ogg", ".wav", ".flac"}

	if indecies == nil {
		return nil
	}

	for _, match := range matches {
		dotidx := bytes.LastIndexByte(match[2], '.')
		if dotidx >= 0 {
			ext := match[2][dotidx:]
			if slices.Contains(VideoExt, string(ext)) {
				fmt.Printf("%q is a video\n", match[0])
			}
			if slices.Contains(AudioExt, string(ext)) {
				fmt.Printf("%q is a audio\n", match[0])
			}
		}
		block.Advance(len(match[0]))
	}

	//return NewMedia(url)
	return ast.NewLink()
}

// RegisterFuncs registers the parser with the Goldmark parser.
//func (p *mediaParser) RegisterFuncs(reg parser.InlineParserFuncRegisterer) {
//	reg.Register(p.Trigger(), p)
//}

// Create Renderer

// var contextKeySnippet = parser.NewContextKey()
//
// type mediaTransformer struct{}
//
//	func (r mediaTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
//		//var buf bytes.Buffer
//		ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
//			if entering && n.Kind() == ast.KindImage {
//				//var imagenode bytes.Buffer
//				img := n.(*ast.Image)
//				dotidx := bytes.LastIndexByte(img.Destination, '.')
//				if dotidx >= 0 {
//					ext := img.Destination[dotidx:]
//					fmt.Printf("%s:\t%s\n", string(img.Destination), string(ext))
//				}
//			}
//			return ast.WalkContinue, nil
//		})
//	}
type mediaExtension struct{}

func (e *mediaExtension) Extend(m goldmark.Markdown) {
	//p := int(^uint(0) >> 1) // Lowest priority
	m.Parser().AddOptions(
		//parser.WithASTTransformers(
		//	util.Prioritized(mediaTransformer, p),
		//),
		parser.WithInlineParsers(
			util.Prioritized(NewMediaParser(), 900),
		),
	)
}

func MediaExtension() goldmark.Extender {
	return &mediaExtension{}
}
