package handler

import "strings"

var dictionary = []string{
	"ベータ",
	"版",
	"テスト",
	"だせい",
	"コミュ",
	"コミュニティ",
	"楽しい",
	"たのしい",
	"大変",
	"最高",
	"疲れた",
	"眠い",
	"仕事",
	"会社",
	"朝",
	"昼",
	"夜",
	"おはよう",
	"こんにちは",
	"こんばんは",
	"ありがとう",
	"よろしく",
	"かわいい",
	"すごい",
	"応援",
	"水分補給",
	"ランチ",
	"骨",
}

func splitWords(text string) []string {
	var words []string

	for len(text) > 0 {

		longest := ""

		for _, w := range dictionary {
			if strings.HasPrefix(text, w) {
				if len(w) > len(longest) {
					longest = w
				}
			}
		}

		if longest != "" {
			words = append(words, longest)
			text = strings.TrimPrefix(text, longest)
			continue
		}
for _, p := range particles {
    if strings.HasPrefix(text, p) {
        text = strings.TrimPrefix(text, p)
        goto NEXT
    }
}
		r := []rune(text)

        end := len(r)
        if end > 6 {
            end = 6
        }

        unknown := string(r[:end])
        words = append(words, unknown)

        text = string(r[end:])
	}
NEXT:
	return words
}
func isParticle(word string) bool {
    for _, p := range particles {
        if word == p {
            return true
        }
    }
    return false
}