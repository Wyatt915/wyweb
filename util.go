package main

import (
	"errors"
	"os"
	"slices"
	"strings"
)

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
