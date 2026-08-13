package handler

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type ExternalWord struct {
	Word string `json:"word"`
	POS  string `json:"pos"`
}

var (
	externalDictionaryMu sync.RWMutex
	externalDictionary   []ExternalWord
)

// 外部辞書を読み込む
func LoadExternalDictionary() error {
	data, err := os.ReadFile("external_dictionary.json")
	if err != nil {
		return err
	}

	var words []ExternalWord

	if err := json.Unmarshal(data, &words); err != nil {
		return err
	}

	externalDictionaryMu.Lock()
	externalDictionary = words
	externalDictionaryMu.Unlock()

	return nil
}

// 外部辞書から、現在の言葉に続けられそうな言葉を探す
// Wikipediaから文脈に関連する文章を取得する
func fetchExternalTexts(context string) ([]string, error) {
	context = strings.TrimSpace(context)
	if context == "" {
		return nil, nil
	}

	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "search")
	params.Set("srsearch", context)
	params.Set("format", "json")
	params.Set("utf8", "1")
	params.Set("srlimit", "5")

	apiURL := "https://ja.wikipedia.org/w/api.php?" + params.Encode()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "project-dasei/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var result struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var texts []string

	for _, item := range result.Query.Search {
		if item.Title != "" {
			texts = append(texts, item.Title)
		}

		if item.Snippet != "" {
			texts = append(texts, item.Snippet)
		}
	}

	return texts, nil
}
func pickExternalCandidate(texts []string, current string) string {
	var candidates []string

	for _, text := range texts {
		text = strings.TrimSpace(text)

		if text == "" {
			continue
		}

		// Wikipediaの検索結果に含まれるHTMLタグを除去
		text = strings.ReplaceAll(text, "<span class=\"searchmatch\">", "")
		text = strings.ReplaceAll(text, "</span>", "")

		// 文章を短いまとまりに分割
		parts := strings.FieldsFunc(text, func(r rune) bool {
			switch r {
			case '、', '。', '！', '？', '!', '?', '・',
				'「', '」', '（', '）', '(', ')', ' ', '\n', '\t':
				return true
			}
			return false
		})

		for _, part := range parts {
			part = strings.TrimSpace(part)

			if part == "" || part == current {
				continue
			}

			if isProtectedName(part) {
				continue
			}

			if isMeaninglessLearnedText(part) {
				continue
			}

			runes := []rune(part)

			// 長すぎる検索結果は候補にしない
			if len(runes) < 2 || len(runes) > 12 {
				continue
			}

			// 現在の言葉が候補の近くに出ている場合は
			// 文脈的に関連性が高い候補として優先する
			if strings.Contains(text, current) {
				candidates = append(candidates, part)
				continue
			}

			// 現在の言葉と一緒に検索結果へ出てきた候補
			candidates = append(candidates, part)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// 完全にランダムではなく、
	// 候補の中からランダムに選ぶことで予測不能性を残す
	return candidates[rand.Intn(len(candidates))]
}
func findExternalNextWord(word string) (string, bool) {
	// まずネットから文脈候補を探す
	texts, err := fetchExternalTexts(word)
	if err == nil {
		if candidate := pickExternalCandidate(texts, word); candidate != "" {
			return candidate, true
		}
	}

	externalDictionaryMu.RLock()
	defer externalDictionaryMu.RUnlock()

	if len(externalDictionary) == 0 {
		return "", false
	}

	// 現在の言葉の品詞を調べる
	currentPOS := ""
	for _, item := range externalDictionary {
		if strings.TrimSpace(item.Word) == word {
			currentPOS = strings.TrimSpace(item.POS)
			break
		}
	}

	// 現在の品詞から、次に自然につながりやすい品詞を決める
	preferredPOS := map[string][]string{
		"noun":      {"phrase", "adjective", "verb"},
		"phrase":    {"adjective", "verb", "phrase"},
		"adjective": {"phrase", "adverb", "verb"},
		"adverb":    {"verb", "adjective", "phrase"},
		"verb":      {"phrase", "adverb", "adjective"},
	}

	preferred := preferredPOS[currentPOS]
	// 特定の言葉同士で自然につながる組み合わせを優先する
	preferredWords := map[string][]string{
		"今日は":   {"暑いね", "ゆっくり", "休もう"},
		"暑いね":   {"ゆっくり", "休もう"},
		"ゆっくり":  {"休もう"},
		"おもしろい": {"なるほど", "そうなんだ"},
		"なるほど":  {"そうなんだ"},
	}

	// 言葉そのものの相性を最優先する
	if words := preferredWords[word]; len(words) > 0 {
		var wordCandidates []string

		for _, preferredWord := range words {
			for _, item := range externalDictionary {
				w := strings.TrimSpace(item.Word)

				if w == preferredWord && !isProtectedName(w) {
					wordCandidates = append(wordCandidates, w)
					break
				}
			}
		}

		if len(wordCandidates) > 0 {
			return wordCandidates[rand.Intn(len(wordCandidates))], true
		}
	}
	if len(preferred) == 0 {
		// 外部辞書に存在しない未知の言葉は、
		// 名詞・フレーズとして自然につながる候補を優先する
		preferred = []string{"phrase", "adjective", "verb"}
	}

	// まず、相性のいい品詞だけで候補を作る
	var candidates []string

	for _, item := range externalDictionary {
		w := strings.TrimSpace(item.Word)
		pos := strings.TrimSpace(item.POS)

		if w == "" || w == word {
			continue
		}

		if isProtectedName(w) {
			continue
		}

		for _, p := range preferred {
			if pos == p {
				candidates = append(candidates, w)
				break
			}
		}
	}

	// 相性のいい候補がなければ、従来どおり全体から選ぶ
	if len(candidates) == 0 {
		for _, item := range externalDictionary {
			w := strings.TrimSpace(item.Word)

			if w == "" || w == word {
				continue
			}

			if isProtectedName(w) {
				continue
			}

			candidates = append(candidates, w)
		}
	}

	if len(candidates) == 0 {
		return "", false
	}

	return candidates[rand.Intn(len(candidates))], true
}
