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

	// 40%だけ荒ぶる
	if rand.Intn(100) >= 40 {
		return post
	}

	var result []string
	length := 0
	lastWord := ""
	result = append(result, post)
	length += len([]rune(post))

	for length < 140 {

		switch rand.Intn(10) {

		case 0, 1, 2, 3:
			post := generateMemoryPost()
			if post != "" {
				result = append(result, randomPhrase(post))
			}

		case 4, 5:
			name := randomMember()
			if name != "" {
				result = append(result, randomPhrase(name))
			}

		default:
			words := crazyWords()
			result = append(result, randomPhrase(words[rand.Intn(len(words))]))
		}
		last := result[len(result)-1]

		if last == lastWord {
			continue
		}

		lastWord = last
		length += len([]rune(last))
	}

	// 1行に2～4フレーズ入れて4～5行くらいにまとめる
	var lines []string
	line := ""

	for _, word := range result {

		if line == "" {
			line = word
		} else {
			line += " " + word
		}

		// 2～4フレーズごとに改行
		if rand.Intn(3)+2 <= len(strings.Fields(line)) {
			lines = append(lines, line)
			line = ""
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	// 行数が多すぎる場合は最後の行にまとめる
	for len(lines) > 5 {
		lines[len(lines)-2] += " " + lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}

	return polishDaseiReply(strings.Join(lines, "\n"))
}
