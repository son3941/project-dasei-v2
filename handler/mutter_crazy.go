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

func createCrazyMutter(
	communityID string,
	post string,
) string {
	if rand.Intn(100) >= 40 {
		return post
	}

	communityID = strings.TrimSpace(communityID)
	post = strings.TrimSpace(post)

	if communityID == "" || post == "" {
		return ""
	}

	var result []string

	result = append(result, post)

	// このコミュで覚えている言葉を素材として混ぜる。
	if rand.Intn(100) < 60 {
		memoryPost := generateMemoryPost(
			communityID,
		)

		if memoryPost != "" {
			result = append(
				result,
				strings.TrimSpace(memoryPost),
			)
		}
	}

	// 荒ぶる短い要素を少量だけ混ぜる。
	if rand.Intn(100) < 70 {
		words := crazyWords()

		result = append(
			result,
			words[rand.Intn(len(words))],
		)
	}

	text := strings.TrimSpace(
		strings.Join(result, " "),
	)

	return text
}
