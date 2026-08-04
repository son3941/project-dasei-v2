package handler

import (
	"math/rand"
	"strings"
)

func buildSentence(subject, predicate string) string {
	predicate = strings.TrimSpace(predicate)
	// predicate が助詞で始まるなら、そのまま繋ぐ
	if strings.HasPrefix(predicate, "は") ||
		strings.HasPrefix(predicate, "が") ||
		strings.HasPrefix(predicate, "を") ||
		strings.HasPrefix(predicate, "に") ||
		strings.HasPrefix(predicate, "で") ||
		strings.HasPrefix(predicate, "って") ||
		strings.HasPrefix(predicate, "も") {

		return subject + predicate
	}
	// 文章っぽいものはそのまま返す
	if strings.Contains(subject, " ") ||
		strings.Contains(subject, "、") ||
		strings.Contains(subject, "。") {

		return subject
	}
	// predicate が長い文章ならそのまま使う
	if len([]rune(predicate)) > 15 {
		return subject + " " + predicate
	}
	if strings.Contains(predicate, "は") ||
		strings.Contains(predicate, "が") ||
		strings.Contains(predicate, "やねん") ||
		strings.Contains(predicate, "なの❤") ||
		strings.Contains(predicate, "かな") ||
		strings.Contains(predicate, "！") ||
		strings.Contains(predicate, "？") {

		return subject + " " + predicate
	}
	simple := []string{
		subject + "好き",
		subject + "気になる",
		subject + "ええね",
		subject + "ゴイスー",
		subject + "かな",
	}

	join := []string{
		subject + "は" + predicate,
		subject + "も" + predicate,
		subject + "って" + predicate,
		subject + "で" + predicate,
	}

	ending := []string{
		predicate + "だよ",
		predicate + "かも",
		predicate + "らしい",
		predicate + "かな",
	}
	r := rand.Intn(100)

	switch {

	case r < 60:
		return join[rand.Intn(len(join))]

	case r < 85:
		return ending[rand.Intn(len(ending))]

	default:
		return simple[rand.Intn(len(simple))]
	}
}
