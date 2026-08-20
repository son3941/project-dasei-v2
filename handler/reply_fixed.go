package handler

import (
	"log/slog"
	"strings"
)

func mentionReply(text string) string {
	slog.Info("mention reply",
		slog.String("text", text),
	)

	// 自分へのメンション部分を取り除く
	text = strings.ReplaceAll(text, "@dasei", "")
	text = strings.ReplaceAll(text, "@惰性", "")
	text = strings.TrimSpace(text)

	// メンションだけだった場合
	if text == "" {
		return addEmoji("呼んだだせい？")
	}

	// メンションを除いた本文で通常の返信処理を行う
	if reply := fixedReply(text); reply != "" {
		return finalizeDaseiReply(text, reply)
	}

	return createReply(text)
}
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
