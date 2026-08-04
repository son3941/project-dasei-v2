package handler

import (
	"math/rand"
	"strings"
)

func randomStyle(text string) string {

	switch rand.Intn(100) {

	case 0, 1, 2:
		return zombieStyle(text)

	case 3, 4, 5:
		return ojisanStyle(text)

	default:
		return text
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
	text = strings.ReplaceAll(text, "カナ", "かな❓")
	text = strings.ReplaceAll(text, "です", "ですヨ😊")
	text = strings.ReplaceAll(text, "！", "❗🤣")

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
	greetings := []string{
		"",
		"お疲れ様😊\n",
		"やっほー😆\n",
		"元気かナ❓😊\n",
	}

	endings = []string{
		"",
		"\n今日も頑張ろうネ😊",
		"\n無理しちゃダメだヨ🤣",
		"\nまた話そうネ✨",
		"\n返信待ってるヨ😆",
	}
	return greetings[rand.Intn(len(greetings))] +
		text +
		endings[rand.Intn(len(endings))]
}
