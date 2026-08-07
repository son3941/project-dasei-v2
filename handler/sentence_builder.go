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
		subject + "最高",
		subject + "ｲｲﾈ",
		subject + "好きかも",
		subject + "よき",
		"やっぱり" + subject,
	}

	join := []string{
		subject + "は" + predicate + "だよ",
		subject + "って" + predicate + "だよ",
		subject + "も" + predicate + "だね",
		subject + "で" + predicate + "らしい",
		subject + "気になる😊",
		subject + "最高😆",
		subject + "好き💕",
		subject + "いいね✨",
		subject + "なら" + predicate,
		predicate + "な" + subject,
		predicate + "の" + subject,
		subject + "しか勝たん",
	}
	ending := []string{
		predicate + "だよ",
		predicate + "かも",
		predicate + "らしい",
		predicate + "かな",
		predicate + "最高😊",
	}
	r := rand.Intn(100)

	switch {

	case r < 60:
		return pickGood(join)

	case r < 85:
		return pickGood(ending)

	default:
		return pickGood(simple)
	}
}
func isBadEnding(s string) bool {
	bad := []string{
		"ちゃっ",
		"だっ",
		"て",
		"で",
		"を",
		"は",
		"が",
		"に",
		"と",
		"も",
		"へ",
		"や",
		"か",
	}

	for _, b := range bad {
		if strings.HasSuffix(s, b) {
			return true
		}
	}
	return false
}
func pickGood(list []string) string {
	for i := 0; i < 10; i++ {
		s := list[rand.Intn(len(list))]
		if !isBadEnding(s) {
			return s
		}
	}

	return list[rand.Intn(len(list))]
}
