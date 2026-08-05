package handler

import (
	"fmt"
	"math/rand"
	"strings"
)

func randomStyle(text string) string {

	r := rand.Intn(100)

	switch {

	case r < 60:
		return text

	case r < 70:
		return ojisanTranslate(text)

	case r < 80:
		return zombieTranslate(text)

	default:
		return chaosTranslate(text)
	}
}

func zombieTranslate(text string) string {

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

func ojisanTranslate(text string) string {

	body := text

	body = strings.ReplaceAll(body, "！", "！😊")
	body = strings.ReplaceAll(body, "?", "❓")
	body = strings.ReplaceAll(body, "？", "❓")
	body = strings.ReplaceAll(body, "かな", "かナ😊")
	body = strings.ReplaceAll(body, "だよ", "だヨ😊")
	body = strings.ReplaceAll(body, "です", "だヨ😊")

	jokes := []string{
		"オジサンも今日はゴロゴロしてたヨ🤣",
		"最近暑いネ〜🥵",
		"ちゃんと寝るんだヨ😪",
		"ラーメン🍜食べたくなっちゃったヨ🤣",
		"オジサンも眠いヨ😪",
		"今日も仕事だったヨ🤣",
		"アイス🍨食べたいネ😊",
		"カレー🍛は飲み物だヨ🤣",
		"水分補給🥤忘れないようにネ😊",
		"無理しちゃダメだヨ✨",
	}

	closings := []string{
		"返信待ってるヨ📩💕",
		"またお話しようネ😊",
		"今日も頑張ろうネ✨",
		"ゆっくり休むんだヨ😪",
		"ナンチャッテ🤣",
	}

	templates := []string{

		`%sチャン😊💕

%s

%s

%s
ナンチャッテ🤣`,

		`やっほ〜😊💕

%s

%s

オジサン心配してるヨ🥺

%s`,

		`今日もお疲れ様😊

%s

%s

%s💕`,

		`元気かナ😊❓

%s

%s

%s✨`,

		`%s😊

最近どうかナ❓🤣

%s

%s

返信くれると嬉しいナ💕`,
	}

	template := templates[rand.Intn(len(templates))]
	joke := jokes[rand.Intn(len(jokes))]
	closing := closings[rand.Intn(len(closings))]

	return fmt.Sprintf(
		template,
		body,
		joke,
		closing,
	)
}
func chaosTranslate(text string) string {

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
