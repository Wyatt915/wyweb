package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

func averageColor(rect *image.Image, x0, y0, x1, y1 int) color.RGBA64 {
	var r, g, b, a int
	var out color.RGBA64
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
	out.R = uint16(r / n)
	out.G = uint16(g / n)
	out.B = uint16(b / n)
	out.A = 0xffff
	return out
}

func scaleImage(img *image.Image) *image.Image {
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
	var out image.Image
	out = image.NewRGBA64(image.Rect(0, 0, width, height))
	var x0, y0 int
	var x1, y1 int
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
	return &out
}

func writeThumbnail(imageFileName string, thumbdir string) {
	defer timer("writeThumbnail for " + imageFileName)()
	imgFile, err := os.Open(imageFileName)
	if err != nil {
		log.Printf("WARN: Could not open %s.\n", imageFileName)
		return
	}
	defer imgFile.Close()
	ext := filepath.Ext(imageFileName)
	basename := filepath.Base(imageFileName)
	nameNoExt := strings.Split(basename, ext)[0]
	var fullImg image.Image
	switch ext {
	case ".jpg", ".jpeg":
		fullImg, err = jpeg.Decode(imgFile)
		if err != nil {
			log.Printf("WARN: could not decode %s as jpeg.\n", imageFileName)
			return
		}
	case ".png":
		fullImg, err = png.Decode(imgFile)
		if err != nil {
			log.Printf("WARN: could not decode %s as png.\n", imageFileName)
			return
		}
	}
	thumbnail := scaleImage(&fullImg)
	thumbFileName := filepath.Join(thumbdir, nameNoExt+".jpg")
	thumbFile, err := os.Create(thumbFileName)
	if err != nil {
		log.Printf("WARN: could not open %s for writing.\n", thumbFileName)
		return
	}
	//var out image.Image
	//
	//draw.Draw(
	jpeg.Encode(thumbFile, *thumbnail, nil)
}

func createThumbnails(path string, images []string) error {
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
		thumbs = append(thumbs, entry.Name())
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
