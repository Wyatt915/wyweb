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
	var r, g, b, a int
	var out color.RGBA
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			tempR, tempG, tempB, tempA := (*rect).At(x, y).RGBA()
			r += int(tempR)
			g += int(tempG)
			b += int(tempB)
			a += int(tempA)
		}
	}
	n := (x1 - x0) * (y1 - y0)
	out.R = uint8(r / n)
	out.G = uint8(g / n)
	out.B = uint8(b / n)
	out.A = uint8(a / n)
	return out
}

func scaleImage(img *image.Image, fast bool) *image.Image {
	var scalefactor float32
	maxWidth := 300
	maxHeight := 300
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
	var out image.Image = image.NewRGBA64(image.Rect(0, 0, width, height))
	var x0, y0 int
	var x1, y1 int
	if fast {
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				x0 = int(float32(origWidth*x) / float32(width))
				y0 = int(float32(origHeight*y) / float32(height))
				out.(draw.Image).Set(x, y, (*img).At(x0, y0))
			}
		}
	} else {
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				x0 = int(float32(origWidth*x) / float32(width))
				y0 = int(float32(origHeight*y) / float32(height))
				x1 = int(float32(origWidth*(x+1)) / float32(width))
				y1 = int(float32(origHeight*(y+1)) / float32(height))
				//sub := (*img).(*image.RGBA64).SubImage(image.Rect(x0, y0, x1, y1))
				//(*(out.(*image.RGBA64))).Set(x, y, averageColor(sub.(*image.RGBA64)))
				//out.(draw.Image).Set(x, y, (*img).At(x0, y0))
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
	//_, format, err := image.DecodeConfig(imgFile)
	//if err != nil {
	//	log.Printf("WARN: could not decode %s as any known image type.\n", imageFileName)
	//	return
	//}
	var fullImg image.Image
	//switch format {
	//case "jpeg":
	//	fullImg, err = jpeg.Decode(imgFile)
	//	if err != nil {
	//		log.Printf("WARN: could not decode %s as jpeg.\n", imageFileName)
	//		log.Println(err)
	//		return
	//	}
	//case "png":
	//	fullImg, err = png.Decode(imgFile)
	//	if err != nil {
	//		log.Printf("WARN: could not decode %s as png.\n", imageFileName)
	//		log.Println(err)
	//		return
	//	}
	//default:
	//	log.Printf("%s has unknown format %s\n", imageFileName, format)
	//}
	fullImg, _, err = image.Decode(imgFile)
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
	//var out image.Image
	//
	//draw.Draw(
	println(thumbFileName, png.Encode(thumbFile, *thumbnail))
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
func pairUp(path string, fullsized []string) []imgPair {
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
		log.Printf("Loss: %f", loss)
		//		galleryRow := arr.AppendNew("div", Class("galleryrow"))
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
				sw := imgSwap{coords[i], coords[j], 0}
				improvement := trySwap(&grid, sw, target)
				if improvement < bestSwap.score {
					keepGoing = true
					bestSwap = imgSwap{coords[i], coords[j], improvement}
				}
			}
		}

		//for _, col := range grid {
		//	galleryCol := galleryRow.AppendNew("div", Class("gallerycol"))
		//	for idx, pair := range col {
		//		attr := map[string]string{
		//			"src":            pair.Thumb,
		//			"id":             fmt.Sprintf("imgseq-%d", idx),
		//			"data-image-num": strconv.Itoa(idx),
		//			"data-fullsize":  pair.Full,
		//		}
		//		galleryCol.AppendNew("img", Class("gallery-image"), attr)
		//	}
		//}
		if keepGoing {
			if bestMove.score < bestSwap.score {
				loss = doMove(&grid, bestMove, target)
			} else if bestSwap.score < bestMove.score {
				loss = doSwap(&grid, bestSwap, target)
			}
		}
		//	arr.AppendNew("hr")
	}
	log.Printf("Arrangement completed after %d iterations\n", counter)
	return grid
}

// try to minimize loss on the first pass of filling the grid
//func prefill(pairs []imgPair, columns int) [][]imgPair {
//	heights := make([]float32, columns)
//	out := make([][]imgPair, columns)
//	for i := 0; i < columns; i++ {
//		out[i] = make([]imgPair, 0)
//	}
//	var totalHeight float32
//	var target float32
//	for _, pair := range pairs {
//		totalHeight += pair.Aspect
//		target = totalHeight / float32(columns)
//		var loss float32
//		var record float32 = math.MaxFloat32
//		var choice int
//		for i := range columns {
//			heights[i] = calcTotalHeight(out[i])
//		}
//		for i := range columns {
//			loss = 0
//			for j := range columns {
//				difference := heights[i] - target
//				if i == j {
//					difference += pair.Aspect
//				}
//				loss += difference * difference
//			}
//			if loss < record {
//				record = loss
//				choice = i
//			}
//		}
//		out[choice] = append(out[choice], pair)
//	}
//	log.Printf("Initial Loss: %f", calcLoss(&out, target))
//	return out
//}

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
	sublists := partition([][]imgPair{pairs}, 32)
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
	//prep := prefill(pairs, columns)
	return out
}

func gallery(node *wmd.ConfigNode) {
	extensions := []string{"jpg", "png"}
	fullsized := findImages(node.Path, extensions)
	createThumbnails(node.Path, fullsized)
	pairs := pairUp(node.Path, fullsized)
	main := NewHTMLElement("main", Class("imagegallery"))
	grid := arrangeImages(pairs, 6, main)
	galleryElem := main.AppendNew("div", Class("gallery"))
	galleryRow := galleryElem.AppendNew("div", Class("galleryrow"))
	for _, col := range grid {
		galleryCol := galleryRow.AppendNew("div", Class("gallerycol"))
		for idx, pair := range col {
			attr := map[string]string{
				"src":            pair.Thumb,
				"id":             fmt.Sprintf("imgseq-%d", idx),
				"data-image-num": strconv.Itoa(idx),
				"data-fullsize":  pair.Full,
			}
			galleryCol.AppendNew("img", Class("gallery-image"), attr)
		}
	}

	node.Resolved.HTML = main
}
