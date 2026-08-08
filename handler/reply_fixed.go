package handler

import "strings"

func fixedReply(text string) string {

	t := strings.TrimSpace(text)

	// おはよう
	if strings.Contains(t, "おはよう") ||
		strings.Contains(t, "おはよ") {
		return addEmoji("おはよう！")
	}

	// がんばる
	if strings.Contains(t, "頑張る") ||
		strings.Contains(t, "頑張ります") ||
		strings.Contains(t, "がんばる") ||
		strings.Contains(t, "がんばります") {
		return addEmoji("がんばれーっ！")
	}

	// おつかれ
	if strings.Contains(t, "仕事終わった") ||
		strings.Contains(t, "仕事終わりました") ||
		strings.Contains(t, "シゴオワ") ||
		strings.Contains(t, "しごおわ") ||
		strings.Contains(t, "疲れた") ||
		strings.Contains(t, "つかれた") {
		return addEmoji("おつかれさまー！")
	}

	return ""
}
