package handler

import (
	"math/rand"
	"strings"
)

func buildSentence(subject, predicate string) string {
	predicate = strings.TrimSpace(predicate)

	if strings.Contains(predicate, "は") ||
		strings.Contains(predicate, "が") ||
		strings.Contains(predicate, "です") ||
		strings.Contains(predicate, "だよ") ||
		strings.Contains(predicate, "かな") ||
		strings.Contains(predicate, "！") ||
		strings.Contains(predicate, "？") {

		return subject + " " + predicate
	}
	patterns := []string{
		subject + "は" + predicate,
		subject + "も" + predicate,
		subject + "って" + predicate,

		subject + "好き",
		subject + "なんやて",
		subject + "ええやん",
		subject + "すんごい",

		predicate + "なの❤",
		predicate + "かも",
		predicate + "らしい",
		predicate + "かな",
		predicate + "好き",
		predicate + "マジで",

		subject + "だと思う",
		subject + "なんだよね",
		subject + "もある",
		subject + "で草",
		subject + "なの？",
		subject + "かもしれない",

		subject + "って" + predicate + "やねん",
		subject + "は" + predicate + "らしい",
		subject + "も" + predicate + "かな",
	}

	return patterns[rand.Intn(len(patterns))]
}
