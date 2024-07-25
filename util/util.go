package util

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

var urlRegex = regexp.MustCompile(`^((http|ftp|https)://)?([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:/~+#-]*[\w@?^=%&/~+#-])?`)

func Timer(name string) func() {
	start := time.Now()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	return func() {
		fmt.Printf("%s:%d %s [%v]\n", filepath.Base(file), line, name, time.Since(start))
	}
}

func PathToList(path string) []string {
	return strings.Split(string(path), string(os.PathSeparator))
}

// NearestCommonAncestor returns the longest path common to both pathA and pathB
func NearestCommonAncestor(pathA []string, pathB []string) ([]string, error) {
	result := make([]string, 0)
	var status error
	for idx, name := range pathA {
		if pathB[idx] != name {
			break
		}
		result = append(result, name)
	}
	if len(result) < 1 {
		status = errors.New("paths do not share an ancestor")
	}
	return result, status
}

// let B = {b ∈ sliceB | b ∉ sliceA} then sliceA ∪ B is equivalent to ConcatUnique(sliceA, sliceB)
func ConcatUnique[T comparable](sliceA []T, sliceB []T) []T {
	result := make([]T, len(sliceA))
	copy(result, sliceA)
	for _, val := range sliceB {
		if !slices.Contains(sliceA, val) {
			result = append(result, val)
		}
	}
	return result
}

func TrimMagicSuffix(str string) string {
	suffixes := []string{
		".post.md",
		".listing",
	}
	out := strings.Clone(str)
	for _, x := range suffixes {
		out = strings.TrimSuffix(out, x)
	}
	return out
}

func RewriteURLPath(url, subdir string) (string, error) {
	url = strings.TrimLeft(url, string(os.PathSeparator))
	//first look in the most local context
	path := filepath.Join(subdir, url)
	_, err := os.Stat(path)
	if err == nil {
		return "/" + path, nil
	}
	// if that fails, try document root as the parent directory
	_, err = os.Stat(url)
	if err == nil {
		return "/" + url, nil
	}
	// finally, see if it looks like a url
	if urlRegex.Match([]byte(url)) {
		return url, nil
	}
	// if everything fails, leave it unchanged but complain about it.
	return url, fmt.Errorf("unknown URL destination %s in directory %s", url, subdir)
}
