package handler

import "strings"

func replyFromTemplate(text string) string {

	switch {

	case strings.Contains(text, "こんにちは"):
		return addEmoji(randomReply(
			"はろー",
			"おお",
			"んー？",
			"はい",
		))

	case strings.Contains(text, "おはよう"):
		return addEmoji(randomReply(
			"おはよう",
			"おお",
			"はろー",
		))

	case strings.Contains(text, "こんばんは"):
		return addEmoji(randomReply(
			"こんばんは",
			"わかる",
			"おお",
		))
	}

	return ""
}
