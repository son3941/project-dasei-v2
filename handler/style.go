package handler

import (
	"math/rand"
	"strings"
)

func randomStyle(text string) string {

	mode := rand.Intn(100)

	switch {
	case mode < 60:
		return text

	case mode < 70:
		return ojisanStyle(text)

	case mode < 80:
		return zombieStyle(text)

	default:
		return chaosStyle(text)
	}
}

func zombieStyle(text string) string {

	prefix := []string{
		"これ",
		"いや",
		"ちなみに",
		"普通に",
		"自分も",
	}

	suffix := []string{
		"🥺",
		"😭",
		"🍆",
		"www",
		"なんよ",
	}

	return prefix[rand.Intn(len(prefix))] +
		text +
		suffix[rand.Intn(len(suffix))]
}

func ojisanStyle(text string) string {

	text = strings.ReplaceAll(text, "ね", "ネ😊")
	text = strings.ReplaceAll(text, "よ", "ヨ😆")
	text = strings.ReplaceAll(text, "かな", "かな❓")
	text = strings.ReplaceAll(text, "です", "ですヨ😊")

	endings := []string{
		"",
		"😊",
		"🤣",
		"✨",
		"👍",
		"ナンチャッテ！",
	}

	text += endings[rand.Intn(len(endings))]

	if rand.Intn(100) < 30 {
		text += "\n今日も頑張ろうネ😊"
	}

	return text
}
func chaosStyle(text string) string {

	mode := rand.Intn(100)

	switch {

	// 英語 30%
	case mode < 30:

		english := []string{
			"Stay strong 💜🩵",
			"Amazing ❤️",
			"Good 👍",
			"Nice work 🦋",
			"Beautiful 💙",
			"WoW",
		}

		return english[rand.Intn(len(english))]

	// 日本語 30%
	case mode < 60:

		japanese := []string{
			"私は好きです",
			"とても美味",
			"よく書かれています",
			"それはどうですか？",
			"最高ですか",
			"それはとてもありがとう",
		}

		return japanese[rand.Intn(len(japanese))]

	// 他言語 20%
	case mode < 80:

		foreign := []string{
			"Gracias 💜",
			"Bonjour 🦋",
			"Merci ❤️",
			"Ciao 😊",
			"Excelente 💙",
		}

		return foreign[rand.Intn(len(foreign))]

	// 絵文字だけ 20%
	default:

		emoji := []string{
			"🦋💜🩵❤️",
			"🥺😭💙",
			"😂😂😂",
			"🍆🍆🍆",
			"🙏✨",
			"👀🔥",
		}

		return emoji[rand.Intn(len(emoji))]
	}
}
