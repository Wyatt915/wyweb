///////////////////////////////////////////////////////////////////////////////////////////////////
//                                                                                               //
//                                                                                               //
//         oooooo   oooooo     oooo           oooooo   oooooo     oooo         .o8               //
//          `888.    `888.     .8'             `888.    `888.     .8'         "888               //
//           `888.   .8888.   .8' oooo    ooo   `888.   .8888.   .8' .ooooo.   888oooo.          //
//            `888  .8'`888. .8'   `88.  .8'     `888  .8'`888. .8' d88' `88b  d88' `88b         //
//             `888.8'  `888.8'     `88..8'       `888.8'  `888.8'  888ooo888  888   888         //
//              `888'    `888'       `888'         `888'    `888'   888    .o  888   888         //
//               `8'      `8'         .8'           `8'      `8'    `Y8bod8P'  `Y8bod8P'         //
//                                .o..P'                                                         //
//                                `Y8P'                                                          //
//                                                                                               //
//                                                                                               //
//                              Copyright (C) 2024  Wyatt Sheffield                              //
//                                                                                               //
//                 This program is free software: you can redistribute it and/or                 //
//                 modify it under the terms of the GNU General Public License as                //
//                 published by the Free Software Foundation, either version 3 of                //
//                      the License, or (at your option) any later version.                      //
//                                                                                               //
//                This program is distributed in the hope that it will be useful,                //
//                 but WITHOUT ANY WARRANTY; without even the implied warranty of                //
//                 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the                 //
//                          GNU General Public License for more details.                         //
//                                                                                               //
//                   You should have received a copy of the GNU General Public                   //
//                         License along with this program.  If not, see                         //
//                                <https://www.gnu.org/licenses/>.                               //
//                                                                                               //
//                                                                                               //
///////////////////////////////////////////////////////////////////////////////////////////////////

package wyweb

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"wyweb.site/util"
)

// Given a path and a list of file extensions ext, return all image filenames in the path.
func findImages(path string) []string {
	stat, err := os.Stat(path)
	if err != nil || !stat.IsDir() {
		return nil
	}
	result := make([]string, 0)
	files, _ := os.ReadDir(path)
	for _, file := range files {
		f, err := os.Open(filepath.Join(path, file.Name()))
		if err != nil {
			continue
		}
		_, format, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			continue
		}
		if format != "" {
			result = append(result, filepath.Join(path, file.Name()))
		}
	}
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
	if origHeight <= maxHeight && origWidth <= maxWidth {
		return img
	}
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
	//ext := filepath.Ext(basename)
	//nameNoExt := strings.Split(basename, ext)[0]
	var fullImg image.Image
	fullImg, _, err = image.Decode(imgFile)
	if err != nil {
		log.Println(err)
	}
	thumbnail := scaleImage(&fullImg, true)
	thumbFileName := filepath.Join(thumbdir, basename+".png")
	thumbFile, err := os.Create(thumbFileName)
	if err != nil {
		log.Printf("WARN: could not open %s for writing.\n", thumbFileName)
		return
	}
	defer thumbFile.Close()
	png.Encode(thumbFile, *thumbnail)
}

func removeExt(name string) string {
	dot := 0
	for idx, char := range name {
		if char == '.' {
			dot = idx
		}
	}
	return name[:dot]
}

func createThumbnails(path string, images []string) error {
	defer util.Timer("createThumbnails")()
	thumbdir := filepath.Join(path, "thumbs")
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
		if slices.Contains(thumbs, basename) {
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
	thumbfiles, _ := os.ReadDir(thumbdir)
	//unmatchedRemaining = true
	//for unmatchedRemaining {
	//unmatchedRemaining = false
	for _, entry := range thumbfiles {
		thumbMap[removeExt(entry.Name())] = filepath.Join(thumbdir, entry.Name())
	}
	matches := make([]string, 0) //A match is a thumbnail with a corresponding fullsized image
	for _, full := range fullsized {
		thumb, ok := thumbMap[filepath.Base(full)]
		if !ok {
			log.Printf("Could not find thumb for %s", full)
			continue
		}
		matches = append(matches, thumb)
		result = append(result, imgPair{Full: full, Thumb: thumb})
	}
	//orphans := make([]string, 0) //An orphan is a thumbnail withourt a corresponding fullsized image
	for _, entry := range thumbfiles {
		if !slices.Contains(matches, filepath.Join(thumbdir, entry.Name())) {
			log.Printf("Removing %s", filepath.Join(thumbdir, entry.Name()))
			os.Remove(filepath.Join(thumbdir, entry.Name()))
		}
	}
	//}
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
func tryMove(grid [][]imgPair, mv imgMove, target float32) float32 {
	diffs := make([]float32, len(grid))
	for i := range grid {
		diffs[i] = calcTotalHeight(grid[i]) - target
	}
	src, dst := mv.pos.x, mv.col
	height := grid[mv.pos.x][mv.pos.y].Aspect
	diffs[src] -= height
	diffs[dst] += height
	var loss float32
	for _, height := range diffs {
		loss += height * height
	}
	return loss
}

func doSwap(grid *[][]imgPair, sw imgSwap, target float32) float32 {
	(*grid)[sw.posA.x][sw.posA.y], (*grid)[sw.posB.x][sw.posB.y] = (*grid)[sw.posB.x][sw.posB.y], (*grid)[sw.posA.x][sw.posA.y]
	return calcLoss(grid, target)
}

func trySwap(grid [][]imgPair, sw imgSwap, target float32) float32 {
	diffs := make([]float32, len(grid))
	for i := range grid {
		diffs[i] = calcTotalHeight(grid[i]) - target
	}
	colA, colB := sw.posA.x, sw.posB.x
	heightA, heightB := grid[sw.posA.x][sw.posA.y].Aspect, grid[sw.posB.x][sw.posB.y].Aspect
	diffs[colA] += heightB - heightA
	diffs[colB] += heightA - heightB
	var loss float32
	for _, height := range diffs {
		loss += height * height
	}
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

				improvement := tryMove(grid, imgMove{pos, col, 0}, target)
				if improvement < bestMove.score {
					keepGoing = true
					bestMove = imgMove{pos, col, improvement}
				}
			}
			for j := i + 1; j < len(coords); j++ {
				sw := imgSwap{coords[i], coords[j], 0}
				improvement := trySwap(grid, sw, target)
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
	defer util.Timer("arrangeImages")()
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

func BuildGallery(node *ConfigNode) error {
	fullsized := findImages(node.Path)
	createThumbnails(node.Path, fullsized)
	pairs := PairUp(node.Path, fullsized)
	main := NewHTMLElement("body", Class("gallery-page"))
	header := main.AppendNew("header")
	bcHTML, bcSD := Breadcrumbs(node)
	node.StructuredData = append(node.StructuredData, bcSD)
	header.Append(bcHTML)
	header.AppendNew("h1").AppendText(node.Title)
	header.AppendNew("div", Class("description")).AppendText(node.Description)
	grid := arrangeImages(pairs, 4, main)
	galleryElem := main.AppendNew("div", Class("gallery"))
	galleryRow := galleryElem.AppendNew("div", Class("gallery-row"))
	imageNum := 0

	richImages := make(map[string]RichImage)
	structuredData := make([]interface{}, 0)
	for _, img := range node.Images {
		log.Println(img.Filename)
		if slices.ContainsFunc(fullsized, func(e string) bool { return filepath.Base(e) == img.Filename }) {
			structuredData = append(structuredData, img.StructuredData())
			richImages[img.Filename] = img
		}
	}

	for _, col := range grid {
		galleryCol := galleryRow.AppendNew("div", Class("gallery-col"))
		for _, pair := range col {
			attr := map[string]string{
				"src":            pair.Thumb,
				"data-image-num": strconv.Itoa(imageNum),
				"data-fullsize":  pair.Full,
				"loading":        "lazy",
			}
			if img, ok := richImages[filepath.Base(pair.Full)]; ok {
				attr["id"] = img.GetIDb64()
				attr["alt"] = img.Alt
			}
			galleryCol.AppendNew("img", Class("gallery-image"), attr)
			imageNum++
		}
	}
	node.HTML = main
	imgSD, err := json.MarshalIndent(structuredData, "", "    ")
	if err != nil {
		log.Println(err.Error())
	} else {
		node.StructuredData = append(node.StructuredData, string(imgSD))
	}

	return nil
}

func GetGalleryInfo(node *ConfigNode, infoReqs []string) ([]byte, error) {
	results := make(map[string]RichImage)

	for _, req := range infoReqs {
		for _, img := range node.Images {
			if id := img.GetIDb64(); id == req {
				log.Println(img.Filename)
				results[id] = img
				break
			}
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no images found")
	}
	return json.Marshal(results)
}
