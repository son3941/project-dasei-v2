package handler

import (
	"fmt"
	"math/rand"
	"strings"
)

func randomStyle(text string) (string, string) {

	r := rand.Intn(100)

	switch {

	case r < 60:
		return text, "normal"

	case r < 70:
		return ojisanTranslate(text), "ojisan"

	case r < 80:
		return zombieTranslate(text), "chuunibyou"

	default:
		return chaosTranslate(text), "chaos"
	}
}
func zombieTranslate(text string) string {

	mode := rand.Intn(100)

	switch {

	// 英語
	case mode < 30:
		return englishZombie()

	// 日本語
	case mode < 60:
		return japaneseZombie()

	// 他言語
	case mode < 80:
		return foreignZombie()

	// 絵文字だけ
	default:
		return emojiZombie()
	}
}
func zombieEnglish() string {
	phrases := []string{
		"Stay strong 🩵💜❤️",
		"Keep going ❤️🩵",
		"Amazing ✨",
		"Beautiful 😊",
		"Excellent 💙",
		"Well done 🦋",
		"Nice 😍",
		"Great job 💜",
		"Fantastic ❤️",
		"Wonderful ✨",
		"Love it 🩵",
		"So good 💙",
	}
	return phrases[rand.Intn(len(phrases))]
}
func zombieJapanese() string {
	phrases := []string{
		"最高です😊",
		"応援しています🩵",
		"素敵ですね✨",
		"いいですね😊",
		"そう思います💜",
		"わかります😊",
		"本当にそう✨",
		"これは良い❤️",
		"ありがとうございます🩵",
		"お疲れ様です😊",
		"すばらしい✨",
		"いい感じですね💙",
	}

	return phrases[rand.Intn(len(phrases))]
}

func englishZombie() string {

	posts := []string{
		"Stay strong 💜🩵❤️",
		"Amazing 🦋💜",
		"Beautiful ❤️🩵",
		"Wonderful 🌸💜",
		"Keep smiling 😊💙",
		"Nice 👍💜",
		"Love this ❤️",
		"Excellent 🩵🦋",
		"Very good 💜✨",
		"Respect ❤️👏",
	}

	return posts[rand.Intn(len(posts))]
}
func japaneseZombie() string {

	posts := []string{
		"応援しています😊",
		"素晴らしいですね✨",
		"私もそう思います💜",
		"最高ですね👏",
		"良いですね😊",
		"素敵です❤️",
		"とても良いです✨",
		"感動しました🥺",
	}

	return posts[rand.Intn(len(posts))]
}
func foreignZombie() string {

	posts := []string{
		"Muy bien ❤️",
		"Excelente 💜",
		"Fantástico 🩵",
		"Bravo 👏",
		"Très bien ❤️",
		"Magnifique 💜",
		"Bellissimo 🩵",
		"Incredible ❤️",
		"Ottimo 💙",
		"Perfecto 🦋",
	}

	return posts[rand.Intn(len(posts))]
}
func emojiZombie() string {

	posts := []string{
		"🦋🩵💜❤️",
		"😭🙏💜",
		"👏👏👏",
		"🔥🔥🔥",
		"💜🩵💜🩵",
		"🥺🥺🥺",
		"❤️❤️❤️",
		"✨✨✨",
		"🤣🤣🤣",
		"👍💙🩵",
	}

	return posts[rand.Intn(len(posts))]
}
func zombieReply(original string) string {

	word := pickWord(strings.Fields(original))
	if word == "" {
		return zombieTranslate("")
	}

	r := rand.Intn(100)

	switch {

	// 70%
	case r < 70:
		return word

	// 20%
	case r < 90:

		emojis := []string{
			"❤️",
			"💜",
			"🩵",
			"🦋",
			"✨",
			"🙏",
			"🥺",
		}

		return word + emojis[rand.Intn(len(emojis))]

	// 10%
	default:

		english := []string{
			"Stay strong",
			"Amazing",
			"Beautiful",
			"Excellent",
			"Wonderful",
			"Respect",
			"Nice",
		}

		return english[rand.Intn(len(english))] + "\n" + word
	}
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

		`やっほ〜😊💕

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

	result := fmt.Sprintf(
		template,
		body,
		joke,
		closing,
	)

	result = finalizeDaseiReply(result, result)

	return result
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
