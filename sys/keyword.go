package sys

import (
	"strings"

	"github.com/surgebase/porter2"
)

type WordList map[string]struct{}

func ParseWordList(content string) WordList {
	words := WordList{}
	for _, word := range strings.Fields(content) {
		words[porter2.Stem(word)] = struct{}{}
	}
	return words
}

func (a WordList) Match(b WordList) bool {
	for w, _ := range a {
		if _, ok := b[w]; ok {
			return true
		}
	}
	return false
}
