package handler

import (
	"fmt"
	"testing"
)

func TestKaibunshoGenerator(t *testing.T) {
	source := "今日は北海道でラーメンを食べた。天気も良くて最高だった。"

	fmt.Println("===== 怪文書ジェネレーター出力テスト =====")
	fmt.Println("元ポスト:", source)
	fmt.Println()

	for i := 1; i <= 10; i++ {
		result := makeKaibunsho(source, nil)

		fmt.Printf("----- %d -----\n", i)
		fmt.Println("Mode:", result.Mode)
		fmt.Println("Level:", result.Level)
		fmt.Println("MixRate:", result.MixRate)
		fmt.Println("ContamRate:", result.ContamRate)
		fmt.Println("Text:", result.Text)
		fmt.Println()
	}
}
