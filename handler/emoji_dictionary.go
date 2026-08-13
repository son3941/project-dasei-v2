package handler

import (
	"bufio"
	"net/http"
	"strings"
	"sync"
)

var (
	emojiOnce sync.Once
	emojiList []string
)

const emojiSourceURL = "https://www.unicode.org/Public/emoji/latest/emoji-test.txt"

// Unicode公式から絵文字一覧を取得する
func loadEmojiDictionary() {
	emojiOnce.Do(func() {
		client := &http.Client{}

		req, err := http.NewRequest(http.MethodGet, emojiSourceURL, nil)
		if err != nil {
			return
		}

		req.Header.Set("User-Agent", "project-dasei/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return
		}

		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			// コメント・空行は無視
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// 例:
			// 1F600 ; fully-qualified     # 😀 E1.0 grinning face
			parts := strings.SplitN(line, "#", 2)
			if len(parts) != 2 {
				continue
			}

			emojiPart := strings.TrimSpace(parts[1])

			if emojiPart == "" {
				continue
			}

			// 「😀 E1.0 grinning face」の先頭だけ取り出す
			fields := strings.Fields(emojiPart)

			if len(fields) == 0 {
				continue
			}

			emoji := fields[0]

			if emoji != "" {
				emojiList = append(emojiList, emoji)
			}
		}
	})
}
