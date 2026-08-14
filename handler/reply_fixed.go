package handler

import (
	"log/slog"
	"strings"
)

func mentionReply(text string) string {
	slog.Info("mention reply",
		slog.String("text", text),
	)

	t := strings.TrimSpace(text)

	// メンション部分を取り除く
	t = strings.TrimSpace(strings.TrimPrefix(t, "だせい"))

	// 「、」「,」などを取り除く
	t = strings.TrimLeft(t, "、, ")

	// まず既存の固定返信を確認
	if reply := fixedReply(t); reply != "" {
		return reply
	}

	// 質問への返信
	if strings.Contains(t, "？") || strings.Contains(t, "?") ||
		strings.Contains(t, "ってどんな") ||
		strings.Contains(t, "って何") ||
		strings.Contains(t, "とは") {

		return addEmoji("うーん、それについて考えてみるね")
	}

	// あいさつ
	if strings.Contains(t, "おはよう") {
		return addEmoji("おはよう！今日もよろしくね")
	}

	if strings.Contains(t, "こんにちは") {
		return addEmoji("こんにちは！")
	}

	if strings.Contains(t, "こんばんは") {
		return addEmoji("こんばんは！")
	}

	// お礼
	if strings.Contains(t, "ありがとう") ||
		strings.Contains(t, "ありがと") {

		return addEmoji("どういたしまして！")
	}

	// 褒められた場合
	if strings.Contains(t, "えらい") ||
		strings.Contains(t, "すごい") ||
		strings.Contains(t, "かわいい") {

		return addEmoji("えへへ、ありがとう！")
	}

	// その他は通常の返信生成へ
	return createReply(t)
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
