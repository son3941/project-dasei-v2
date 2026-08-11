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

	// 現段階では、現在の単語そのものは除外して
	// 外部辞書から候補を作る
	var candidates []string

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

	if len(candidates) == 0 {
		return "", false
	}

	return candidates[rand.Intn(len(candidates))], true
}
