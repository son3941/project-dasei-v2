package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"
const groqModel = "openai/gpt-oss-120b"

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model               string        `json:"model"`
	Messages            []groqMessage `json:"messages"`
	Temperature         float64       `json:"temperature"`
	MaxCompletionTokens int           `json:"max_completion_tokens"`
	ReasoningEffort     string        `json:"reasoning_effort,omitempty"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func generateGroqReply(
	ctx context.Context,
	text string,
	threadPosts []string,
	nickname string,
	memoryHint string,
) (string, error) {

	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is empty")
	}

	text = strings.TrimSpace(text)
	nickname = strings.TrimSpace(nickname)
	memoryHint = strings.TrimSpace(memoryHint)
	if text == "" {
		return "", fmt.Errorf("reply text is empty")
	}

	systemPrompt := `あなたはmixi2のコミュニティbot「だせい」です。

性格:
- 適当で憎めない
- 少し抜けている
- ゆるく親しみやすい
- たまに可愛い返しをする
- 強い言葉や攻撃的な表現は使わない
- AIやアシスタントとして振る舞わず、「だせい」として会話する

返信ルール:
- 相手の最新の発言に自然に返事をする
- 会話履歴がある場合は文脈を理解して返す
- 相手の文章をそのまま言い換えるだけの返信は禁止
- 「〜ってことか」「〜の話なんやな」などの機械的なオウム返しを避ける
- 毎回同じ語尾や似たような構文を使わない
- 必要なら質問、共感、感想、軽い冗談を混ぜる
- 知らないことを知ったふりしない
- 質問されたらだせいらしく回答する
- 文字数は日本語で最大１４０文字
- だせいらしい自然な会話だけを返す
- 返信本文以外の説明、前置き、引用符は出力しない
- たまに日本各地の方言を自然に混ぜる
- 方言を使う場合は、北海道・東北・関東・甲信越・北陸・東海・関西・中国・四国・九州・沖縄など全国からランダムに選ぶ
- 関西弁だけに偏らない
- 一度の返信で複数地域の方言を混ぜない
- 方言は文章全体ではなく、語尾や短い表現に軽く混ぜる程度にする
- 方言を使っても、地域名や方言名を本文に書かない
- 「関東っぽく」「関西弁で」「北海道風」など、方言を説明する注釈を絶対に付けない
- 括弧書きで話し方や方言を説明しない`
	if nickname != "" {
		systemPrompt += "\n- 相手のニックネームは「" +
			nickname +
			"」。今回の返信では必ず一度、会話として自然な位置でこのニックネームを呼ぶ。毎回文頭に置く必要はない。"
	}
	if memoryHint != "" {
		systemPrompt += "\n- だせいが以前覚えた、この発言に関係する記憶があります：「" +
			memoryHint +
			"」。今回の返信では、この記憶を会話として自然に取り入れる。『覚えています』『記憶によると』など説明口調にはしない。記憶の内容をそのまま棒読みせず、だせいらしい自然な返事にする。"
	}
	messages := []groqMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// API消費を抑えるため、履歴は直近8件まで。
	start := 0
	if len(threadPosts) > 8 {
		start = len(threadPosts) - 8
	}

	for _, past := range threadPosts[start:] {
		past = strings.TrimSpace(past)

		if past == "" || past == text {
			continue
		}

		messages = append(messages, groqMessage{
			Role:    "user",
			Content: past,
		})
	}

	messages = append(messages, groqMessage{
		Role:    "user",
		Content: text,
	})

	reqBody := groqRequest{
		Model:               groqModel,
		Messages:            messages,
		Temperature:         0.9,
		MaxCompletionTokens: 180,
		ReasoningEffort:     "low",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf(
			"marshal groq request: %w",
			err,
		)
	}

	requestCtx, cancel := context.WithTimeout(
		ctx,
		10*time.Second,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		groqAPIURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf(
			"create groq request: %w",
			err,
		)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+apiKey,
	)
	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"groq request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(
		io.LimitReader(
			resp.Body,
			1024*1024,
		),
	)
	if err != nil {
		return "", fmt.Errorf(
			"read groq response: %w",
			err,
		)
	}

	if resp.StatusCode < 200 ||
		resp.StatusCode >= 300 {

		return "", fmt.Errorf(
			"groq returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(
				string(responseBody),
			),
		)
	}

	var result groqResponse

	if err := json.Unmarshal(
		responseBody,
		&result,
	); err != nil {

		return "", fmt.Errorf(
			"decode groq response: %w",
			err,
		)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf(
			"groq returned no choices",
		)
	}

	reply := strings.TrimSpace(
		result.Choices[0].Message.Content,
	)

	if reply == "" {
		return "", fmt.Errorf(
			"groq returned empty reply",
		)
	}

	// ニックネームが抜けても再度APIは呼ばない。
	// 使用量節約のためGo側で補完する。
	if nickname != "" &&
		!strings.Contains(reply, nickname) {

		reply = nickname + "、" + reply
	}

	return reply, nil
}
