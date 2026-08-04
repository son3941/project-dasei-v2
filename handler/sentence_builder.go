package handler

import "math/rand"

func buildSentence(subject, predicate string) string {

	patterns := []string{
		subject + "は" + predicate,
		subject + "も" + predicate,
		subject + "って" + predicate,
		predicate + "だよ",
		predicate + "かも",
		predicate + "らしい",
		subject + "かな",
	}

	return patterns[rand.Intn(len(patterns))]
}
