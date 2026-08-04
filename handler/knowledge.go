package handler

func GetKnowledge(keyword string) string {

	switch keyword {

	case "暑い":
		return "暑い日は水分補給が大事なの"

	case "眠い":
		return "眠い時はゆっくり休もう"

	case "雨":
		return "傘を忘れないでね"

	case "地震":
		return "慌てず落ち着いて行動しよう"

	}

	return ""
}
