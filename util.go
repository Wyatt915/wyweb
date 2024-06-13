package main

import (
	"errors"
	"os"
	"strings"
)

func PathToList(path string) []string {
	return strings.Split(string(path), string(os.PathSeparator))
}

func NearestCommonAncestor(a []string, b []string) ([]string, error) {
	result := make([]string, 0)
	var status error
	for idx, name := range a {
		if b[idx] != name {
			break
		}
		result = append(result, name)
	}
	if len(result) < 1 {
		status = errors.New("paths do not share an ancestor")
	}
	return result, status
}
