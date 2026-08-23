package handler

import (
	"math/rand"
	"strings"
)

func crazyWords() []string {
	return []string{
		"あ！！！！",
		"ああ！！！！",
		"おお！！！！",
		"うおおお！！！！",
		"えーとえーと",
		"？？？？",
		"なの！",
		"はい！！！！",
		"わかる！！！！",
		"たぶん！！！！",
		"もそう！！",
		"ふむ……",
		"んーーー",
		"！！！！！",
		"………",
		"うーん",
		"ふふ",
		"だせいもそう",
		"ごろごろ",
		"もぺ",
		"ほんと！？",
	}
}

func createCrazyMutter(post string) string {
	if rand.Intn(100) >= 40 {
		return post
	}

	post = strings.TrimSpace(post)

	if post == "" {
		return ""
	}

	var result []string

	result = append(result, post)
	// 覚えている言葉を素材として混ぜる。
	if rand.Intn(100) < 60 {
		memoryPost := generateMemoryPost()

		if memoryPost != "" {
			result = append(result,
				strings.TrimSpace(memoryPost),
			)
		}
	}

	// 荒ぶる短い要素を少量だけ混ぜる。
	if rand.Intn(100) < 70 {
		words := crazyWords()

		result = append(result,
			words[rand.Intn(len(words))],
		)
	}
	text := strings.TrimSpace(strings.Join(result, " "))

	return finalizeDaseiReply(text, text)
}
