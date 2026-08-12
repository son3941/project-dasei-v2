package handler

import (
	"encoding/json"
	"math/rand"
	"os"
	"strings"
	"sync"
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
func findExternalNextWord(word string) (string, bool) {
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
