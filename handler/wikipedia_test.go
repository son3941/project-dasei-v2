package handler

import "testing"

func TestSearchWikipedia(t *testing.T) {
	result := searchWikipedia("北海道")

	if result == "" {
		t.Fatal("Wikipedia検索結果が空です")
	}

	t.Logf("Wikipedia検索結果: %s", result)
}
