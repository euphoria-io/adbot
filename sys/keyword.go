package sys

import (
	"strings"
	"unicode"

	"github.com/surgebase/porter2"
)

type WordList map[string]struct{}

func ParseWordList(content string) WordList {
	words := WordList{}
	for _, word := range strings.Fields(content) {
		word = strings.ToLower(word)
		word = strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) })
		word = porter2.Stem(word)
		words[word] = struct{}{}
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
