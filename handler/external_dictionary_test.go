package handler

import (
	"os"
	"testing"
)

func TestExternalDictionary(t *testing.T) {
	// テスト実行時の場所をプロジェクト直下に移動
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}

	// 外部辞書を読み込む
	if err := LoadExternalDictionary(); err != nil {
		t.Fatal(err)
	}

	const communityID = "test-community"

	// 外部辞書から次の言葉を取得
	word, ok := findNextWord(
		communityID,
		"今日は",
	)

	if !ok {
		t.Fatal("外部辞書から言葉を取得できませんでした")
	}

	if word == "" {
		t.Fatal("取得した言葉が空です")
	}

	t.Logf("外部辞書から取得した言葉: %s", word)
}

func TestGenerateMemoryPostWithExternalDictionary(t *testing.T) {
	if err := LoadExternalDictionary(); err != nil {
		t.Fatal(err)
	}

	const communityID = "test-community"

	learnedWordsMu.Lock()

	oldPairs := learnedPairs

	learnedPairs = map[string][]LearnedPair{
		communityID: {
			{
				Key:   "今日は",
				Value: "暑いね",
			},
		},
	}

	learnedWordsMu.Unlock()

	defer func() {
		learnedWordsMu.Lock()
		learnedPairs = oldPairs
		learnedWordsMu.Unlock()
	}()

	post := generateMemoryPost(
		communityID,
	)

	if post == "" {
		t.Fatal("generateMemoryPost が空を返しました")
	}

	t.Logf("生成された投稿: %s", post)
}
