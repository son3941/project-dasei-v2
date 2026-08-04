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
		"うおおお",
		"えーとえーと",
		"？？？？",
		"なの！",
		"はい",
		"わかる",
		"たぶん",
		"も",
		"もそう",
		"ふむ",
		"んーー",
		"！！！！！",
		"………",
		"うーん",
		"ふふ",
		"だせいも",
		"ごろごろ",
		"もぺ",
		"ほんと？",
	}
}
func createCrazyMutter(post string) string {

	// 20%だけ荒ぶる
	if rand.Intn(100) >= 20 {
		return post
	}

	var result []string
	length := 0

	result = append(result, post)
	length += len([]rune(post))

	for length < 100 {

		switch rand.Intn(5) {

		case 0:
			post := generateMemoryPost()
			if post != "" {
				result = append(result, post)
			}

		case 1:
			name := randomMember()
			if name != "" {
				result = append(result, randomPhrase(name))
			}

		default:
			words := crazyWords()
			result = append(result, words[rand.Intn(len(words))])
		}

		last := result[len(result)-1]
		length += len([]rune(last))
	}

	return strings.Join(result, "\n\n")
}
