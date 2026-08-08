package handler

import (
	"math/rand"
)

func memberPhrase(name string) string {
	name = nicknameOf(name)

	if name == "" {
		return ""
	}

	switch rand.Intn(8) {

	case 0:
		return addEmoji(name + "も")

	case 1:
		return addEmoji(name + "もそう")

	case 2:
		return addEmoji(name + "もそうだよ")

	default:
		return addEmoji(name)
	}
}
func randomPhrase(text string) string {
	switch rand.Intn(10) {
	case 0:
		return text
	case 1:
		return text + "も"
	case 2:
		return text + "もそう"
	case 3:
		return text + "？"
	case 4:
		return text + "？？"
	case 5:
		return text + "！"
	case 6:
		return text + "！！"
	case 7:
		return text + "..."
	case 8:
		return "だせいも " + text
	default:
		return text
	}
}
