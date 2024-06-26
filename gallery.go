package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	. "wyweb.site/wyweb/html"
	wmd "wyweb.site/wyweb/metadata"
)

// Given a path and a list of file extensions ext, return all image filenames in the path.
func findImages(path string, extensions []string) []string {
	stat, err := os.Stat(path)
	if err != nil || !stat.IsDir() {
		return nil
	}
	result := make([]string, 0)
	for _, ext := range extensions {
		globbed, _ := filepath.Glob(filepath.Join(path, "*."+ext))
		result = slices.Concat(result, globbed)
	}
	//for idx, name := range result {
	//	result[idx] = filepath.Base(name)
	//}
	return result
}

func averageColor(rect *image.Image, x0, y0, x1, y1 int) color.RGBA {
	var r, g, b, a uint32
	var out color.RGBA
	var n uint32
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			tempR, tempG, tempB, tempA := (*rect).At(x, y).RGBA()
			r += uint32(tempR)
			g += uint32(tempG)
			b += uint32(tempB)
			a += uint32(tempA)
			n++
		}
	}
	out.R = uint8(r / n)
	out.G = uint8(g / n)
	out.B = uint8(b / n)
	out.A = uint8(255)
	return out
}

func scaleImage(img *image.Image, fast bool) *image.Image {
	var scalefactor float32
	maxWidth := 600
	maxHeight := 600
	origHeight := (*img).Bounds().Dy()
	origWidth := (*img).Bounds().Dx()
	invHeightRatio := float32(maxHeight) / float32(origHeight)
	invWidthRatio := float32(maxWidth) / float32(origWidth)
	var width, height int
	if invHeightRatio < 1 {
		scalefactor = invHeightRatio
		width = int(scalefactor * float32((*img).Bounds().Dx()))
		height = maxHeight
	} else if invWidthRatio < 1 {
		scalefactor = invWidthRatio
		height = int(scalefactor * float32((*img).Bounds().Dy()))
		width = maxWidth
	} else {
		return img
	}
	var out image.Image = image.NewRGBA(image.Rect(0, 0, width, height))
	var x0, y0 int
	var x1, y1 int
	if fast {
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				x0 = (origWidth * x) / width
				y0 = (origHeight * y) / height
				out.(draw.Image).Set(x, y, (*img).At(x0, y0))
			}
		}
	} else {
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				x0 = (origWidth * x) / width
				y0 = (origHeight * y) / height
				x1 = (origWidth * (x + 1)) / width
				y1 = (origHeight * (y + 1)) / height
				out.(draw.Image).Set(x, y, averageColor(img, x0, y0, x1, y1))
			}
		}
	}
	return &out
}

func writeThumbnail(imageFileName string, thumbdir string) {
	imgFile, err := os.Open(imageFileName)
	if err != nil {
		log.Printf("WARN: Could not open %s.\n", imageFileName)
		return
	}
	defer imgFile.Close()
	basename := filepath.Base(imageFileName)
	ext := filepath.Ext(basename)
	nameNoExt := strings.Split(basename, ext)[0]
	var fullImg image.Image
	var format string
	fullImg, format, err = image.Decode(imgFile)
	println(imgFile, format)
	if err != nil {
		log.Println(err)
	}
	thumbnail := scaleImage(&fullImg, true)
	thumbFileName := filepath.Join(thumbdir, nameNoExt+".png")
	thumbFile, err := os.Create(thumbFileName)
	if err != nil {
		log.Printf("WARN: could not open %s for writing.\n", thumbFileName)
		return
	}
	defer thumbFile.Close()
	png.Encode(thumbFile, *thumbnail)
}

func removeExt(name string) string {
	return strings.Split(name, filepath.Ext(name))[0]
}

func createThumbnails(path string, images []string) error {
	defer timer("createThumbnails")()
	thumbdir := filepath.Join(path, "thumbs")
	println(thumbdir)
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	err = os.Mkdir(thumbdir, stat.Mode())
	if err != nil && !os.IsExist(err) {
		return err
	}
	thumbs := make([]string, 0)
	filepath.WalkDir(thumbdir, func(path string, entry fs.DirEntry, err error) error {
		thumbs = append(thumbs, removeExt(entry.Name()))
		return nil
	})
	var wg sync.WaitGroup
	for _, imageFileName := range images {
		basename := filepath.Base(imageFileName)
		if slices.Contains(thumbs, removeExt(basename)) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			writeThumbnail(imageFileName, thumbdir)
		}()
	}
	wg.Wait()
	return nil
}

type imgPair struct {
	Full   string
	Thumb  string
	Aspect float32 // height / width
}

// Pair up full sized images with their thumbnails
func PairUp(path string, fullsized []string) []imgPair {
	result := make([]imgPair, 0)
	thumbdir := filepath.Join(path, "thumbs")
	thumbMap := make(map[string]string)
	filepath.WalkDir(thumbdir, func(filename string, entry fs.DirEntry, err error) error {
		thumbMap[removeExt(entry.Name())] = filename
		return nil
	})
	for _, full := range fullsized {
		thumb, ok := thumbMap[removeExt(filepath.Base(full))]
		if !ok {
			continue
		}
		result = append(result, imgPair{Full: full, Thumb: thumb})
	}
	for idx, res := range result {
		thumbFile, err := os.Open(res.Thumb)
		if err != nil {
			continue
		}
		config, _, err := image.DecodeConfig(thumbFile)
		if err != nil {
			continue
		}
		result[idx].Aspect = float32(config.Height) / float32(config.Width)
	}
	return result
}

func calcTotalHeight(pairs []imgPair) float32 {
	var totalHeight float32
	for _, img := range pairs {
		totalHeight += img.Aspect
	}
	return totalHeight
}

func calcLoss(grid *[][]imgPair, target float32) float32 {
	var loss float32
	for _, col := range *grid {
		difference := calcTotalHeight(col) - target
		loss += difference * difference
	}
	return loss
}

type coord struct {
	x int
	y int
}

// records which two images are to be swapped
type imgSwap struct {
	posA  coord
	posB  coord
	score float32
}

// records a move from pos to a different column col
type imgMove struct {
	pos   coord
	col   int
	score float32
}

func doMove(grid *[][]imgPair, mv imgMove, target float32) float32 {
	img := (*grid)[mv.pos.x][mv.pos.y]
	(*grid)[mv.pos.x] = append((*grid)[mv.pos.x][:mv.pos.y], (*grid)[mv.pos.x][mv.pos.y+1:]...) // cut
	(*grid)[mv.col] = append((*grid)[mv.col], img)                                              // paste
	return calcLoss(grid, target)
}
func tryMove(grid *[][]imgPair, mv imgMove, target float32) float32 {
	loss := doMove(grid, mv, target)
	img := (*grid)[mv.col][len((*grid)[mv.col])-1]
	(*grid)[mv.col] = (*grid)[mv.col][:len((*grid)[mv.col])-1]
	(*grid)[mv.pos.x] = append((*grid)[mv.pos.x][:mv.pos.y], append([]imgPair{img}, (*grid)[mv.pos.x][mv.pos.y:]...)...)
	return loss
}

func doSwap(grid *[][]imgPair, sw imgSwap, target float32) float32 {
	(*grid)[sw.posA.x][sw.posA.y], (*grid)[sw.posB.x][sw.posB.y] = (*grid)[sw.posB.x][sw.posB.y], (*grid)[sw.posA.x][sw.posA.y]
	return calcLoss(grid, target)
}
func trySwap(grid *[][]imgPair, sw imgSwap, target float32) float32 {
	(*grid)[sw.posA.x][sw.posA.y], (*grid)[sw.posB.x][sw.posB.y] = (*grid)[sw.posB.x][sw.posB.y], (*grid)[sw.posA.x][sw.posA.y]
	loss := calcLoss(grid, target)
	(*grid)[sw.posA.x][sw.posA.y], (*grid)[sw.posB.x][sw.posB.y] = (*grid)[sw.posB.x][sw.posB.y], (*grid)[sw.posA.x][sw.posA.y]
	return loss
}

// Some kind of hill climbing algorithm. Perhaps a good candidate for simulated annealing?
func optimizeArrangement(grid [][]imgPair) [][]imgPair {
	//assume all images have a width of 1. Then the sum of their aspect ratios will be the total height.
	var totalHeight float32
	for _, pairs := range grid {
		totalHeight += calcTotalHeight(pairs)
	}
	target := totalHeight / float32(len(grid))
	loss := calcLoss(&grid, target)
	keepGoing := true
	num := 0
	for _, col := range grid {
		num += len(col)
	}
	coords := make([]coord, num)
	var bestMove imgMove
	var bestSwap imgSwap
	counter := 0
	for keepGoing {
		counter++
		bestMove.score = loss
		bestSwap.score = loss
		keepGoing = false
		num = 0
		for x := 0; x < len(grid); x++ {
			for y := 0; y < len(grid[x]); y++ {
				coords[num] = coord{x, y}
				num++
			}
		}
		for i := range coords {
			pos := coords[i]
			for col := range grid {
				if col == pos.x {
					continue
				}

				improvement := tryMove(&grid, imgMove{pos, col, 0}, target)
				if improvement < bestMove.score {
					keepGoing = true
					bestMove = imgMove{pos, col, improvement}
				}
			}
			for j := i + 1; j < len(coords); j++ {
				//for j := range coords {
				//	if i == j {
				//		continue
				//	}
				sw := imgSwap{coords[i], coords[j], 0}
				improvement := trySwap(&grid, sw, target)
				if improvement < bestSwap.score {
					keepGoing = true
					bestSwap = imgSwap{coords[i], coords[j], improvement}
				}
			}
		}

		if keepGoing {
			if bestMove.score < bestSwap.score {
				loss = doMove(&grid, bestMove, target)
			} else if bestSwap.score < bestMove.score {
				loss = doSwap(&grid, bestSwap, target)
			}
		}
	}
	return grid
}

func prefill(pairs []imgPair, columns int) [][]imgPair {
	prep := make([][]imgPair, columns)
	for i := 0; i < columns; i++ {
		prep[i] = make([]imgPair, 0)
	}
	// First we try to put the same number of images in each column
	n := 0
	imgPerCol := len(pairs) / columns
	for idx, img := range pairs {
		if (idx / (n + 1)) > imgPerCol {
			n++
		}
		prep[n] = append(prep[n], img)
	}
	return prep
}

// bisect the list until each sublist is at most limit.
func partition(parts [][]imgPair, limit int) [][]imgPair {
	out := make([][]imgPair, 0)
	for _, part := range parts {
		n := len(part)
		if n < limit {
			out = append(out, part)
		} else {
			out = append(out, partition([][]imgPair{part[:n/2], part[n/2:]}, limit)...)
		}
	}
	return out
}

func arrangeImages(pairs []imgPair, columns int, page *HTMLElement) [][]imgPair {
	defer timer("arrangeImages")()
	//arr := page.AppendNew("div", Class("gallery"))
	out := make([][]imgPair, columns)
	for i := 0; i < columns; i++ {
		out[i] = make([]imgPair, 0)
	}
	sublists := partition([][]imgPair{pairs}, 64)
	subgrids := make([][][]imgPair, len(sublists))
	var wg sync.WaitGroup
	for i, sublist := range sublists {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prep := prefill(sublist, columns)
			subgrids[i] = optimizeArrangement(prep)
		}()
	}
	wg.Wait()
	for col := range columns {
		for _, grid := range subgrids {
			out[col] = append(out[col], grid[col]...)
		}
	}
	return out
}

func gallery(node *wmd.ConfigNode) {
	extensions := []string{"jpg", "png"}
	fullsized := findImages(node.Path, extensions)
	createThumbnails(node.Path, fullsized)
	pairs := PairUp(node.Path, fullsized)
	main := NewHTMLElement("body", Class("imagegallery"))
	grid := arrangeImages(pairs, 4, main)
	galleryElem := main.AppendNew("div", Class("gallery"))
	galleryRow := galleryElem.AppendNew("div", Class("galleryrow"))
	imageNum := 0
	for _, col := range grid {
		galleryCol := galleryRow.AppendNew("div", Class("gallerycol"))
		for _, pair := range col {
			attr := map[string]string{
				"src":            pair.Thumb,
				"id":             fmt.Sprintf("imgseq-%d", imageNum),
				"data-image-num": strconv.Itoa(imageNum),
				"data-fullsize":  pair.Full,
				"loading":        "lazy",
			}
			galleryCol.AppendNew("img", Class("gallery-image"), attr)
			imageNum++
		}
	}
	node.Resolved.HTML = main
}
