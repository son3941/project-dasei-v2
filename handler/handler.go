package handler

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
)

var fixedWords = []string{
	"だせい",
	"ハムちゃん",
	"そらもよう",
}

type Memory struct {
	Value       string
	LearnedAt   time.Time
	LastReplyAt time.Time

	MutterCount int
}

var (
	members   map[string]string
	membersMu sync.RWMutex
)
var wakati *tokenizer.Tokenizer

func init() {
	members = make(map[string]string)
	var err error
	wakati, err = tokenizer.New(ipa.Dict())
	if err != nil {
		panic(err)
	}
}

var (
	ngMembers = []string{
		"ちー",
	}

	ngAccounts = []string{
		"chiii",
		"chiiii",
		"Chiiiii",
		"Chiiiiii",
		"Chiiiiiii",
		"Chiiiiiiii",
		"Chiiiiiiiii",
	}

	ngWords = []string{
		"ちー",
		"キンカン",
	}
)

type LearnedPair struct {
	Key   string
	Value string
}
type LearnedPhrase struct {
	Text  string
	Count int
}

var (
	memories = make(map[string]Memory)
	memoryMu sync.RWMutex

	nicknames  = make(map[string]string)
	nicknameMu sync.RWMutex

	mutterUsage = make(map[string]int)

	teaches = make(map[string]string)
	teachMu sync.RWMutex
)
var (
	learnedPairs   []LearnedPair
	learnedPhrases []LearnedPhrase
	learnedWordsMu sync.RWMutex
)

// Handler implements event.EventHandler interface.
type Handler struct {
	logger        *slog.Logger
	apiClient     application_apiv1.ApplicationServiceClient
	authenticator auth.Authenticator
	communityID   string
}

// NewHandler creates a new Handler.
func NewHandler(apiClient application_apiv1.ApplicationServiceClient, authenticator auth.Authenticator) *Handler {
	loaded, err := LoadMemories()
	if err == nil {
		memoryMu.Lock()
		for k, v := range loaded {
			memories[k] = Memory{
				Value:     v,
				LearnedAt: time.Now(),
			}
		}
		memoryMu.Unlock()
	}
	pairs, pairErr := LoadLearnedPairs()
	if pairErr != nil {
		slog.Error("LoadLearnedPairs failed",
			slog.String("error", pairErr.Error()),
		)
	} else {
		learnedWordsMu.Lock()
		learnedPairs = pairs
		learnedWordsMu.Unlock()
	}
	return &Handler{
		logger:        slog.Default(),
		apiClient:     apiClient,
		authenticator: authenticator,
	}
}
func (h *Handler) PostMutter(ctx context.Context) error {
	if rand.Intn(100) >= PostChance {
		return nil
	}
	reply := createMutter("")
	reply = randomStyle(reply)
	reply = normalizeDaseiPostLength(reply)

	if reply == "" {
		return nil
	}

	slog.Info("mutter", slog.String("text", reply))

	authCtx, err := h.authenticator.AuthorizedContext(ctx)
	if err != nil {
		return err
	}
	slog.Info("community", slog.String("id", h.communityID))
	if h.communityID == "" {
		return nil
	}
	resp, err := h.apiClient.CreatePost(
		authCtx,
		&application_apiv1.CreatePostRequest{
			CommunityId: &h.communityID,
			Text:        reply,
		},
	)

	if err != nil {
		h.logger.Error("failed to create mutter",
			slog.String("error", err.Error()),
		)
		return err
	}

	slog.Info("create response",
		slog.Any("resp", resp),
	)
	slog.Info("created post id",
		slog.String("postId", resp.Post.PostId),
	)

	slog.Info("★★★★ PostMutter END ★★★★")
	return nil
}

// Handle processes events from mixi2.
func (h *Handler) Handle(ctx context.Context, ev *modelv1.Event) error {
	slog.Info("EVENT TYPE",
		slog.Int("event_type", int(ev.EventType)),
		slog.String("event_id", ev.EventId),
	)
	switch ev.EventType {
	case constv1.EventType_EVENT_TYPE_POST_CREATED:
		post := ev.GetPostCreatedEvent()
		if post == nil {
			return nil
		}
		slog.Info("POST CREATED EVENT",
			slog.Any("post", post),
		)
		h.logger.Info("community post",
			slog.Any("post", post),
		)
		h.logger.Info("event", slog.Any("event", ev))
		h.logger.Info("post data", slog.Any("post", post.GetPost()))
		text := post.GetPost().GetText()

		slog.Info("received text",
			slog.String("text", text),
		)

		if isNGWord(text) {
			return nil
		}

		// リセットコマンド
		if strings.TrimSpace(text) == "だせい リセット実行" {

			memoryMu.Lock()
			memories = make(map[string]Memory)
			memoryMu.Unlock()

			learnedWordsMu.Lock()
			learnedPairs = nil
			learnedWordsMu.Unlock()

			nicknameMu.Lock()
			nicknames = make(map[string]string)
			nicknameMu.Unlock()

			if err := ClearMemories(); err != nil {
				slog.Error("ClearMemories failed",
					slog.String("error", err.Error()),
				)
			} else {
				slog.Info("memories cleared")
			}

			return nil
		}

		displayName := ""
		userID := ""
		if post.GetIssuer() != nil {
			displayName = post.GetIssuer().GetDisplayName()
			userID = post.GetIssuer().GetUserId()

			rememberMember(userID, displayName)
		}

		if displayName == "だせい" {
			return nil
		}
		if isNGMember(displayName) {
			return nil
		}
		if isNGMember(displayName) {
			return nil
		}

		if !isNicknameCommand(text) &&
			!(strings.HasPrefix(text, "だせい、") &&
				strings.Contains(text, "は") &&
				strings.Contains(text, "だよ")) {

			rememberKnowledge(text, displayName)
		}

		if err := SavePost(text); err != nil {
			slog.Error("SavePost failed", slog.String("error", err.Error()))
		}
		account := ""
		if post.GetIssuer() != nil {
			account = post.GetIssuer().GetUserId()
		}
		if isNGAccount(account) {
			return nil
		}

		isMention :=
			strings.Contains(text, "@dasei") ||
				strings.Contains(text, "だせい")

		shouldReply := shouldReplyToPost(text, isMention)

		slog.Info("shouldReply result",
			slog.Bool("shouldReply", shouldReply),
			slog.Bool("isMention", isMention),
		)

		if !shouldReply {
			slog.Info("reply skipped by shouldReplyToPost")
			return nil
		}

		slog.Info("before GenerateReply")
		reply := GenerateReply(
			text,
			isMention,
		)

		reply = normalizeDaseiPostLength(reply)

		if reply == "" {
			return nil
		}
		slog.Info("after GenerateReply",
			slog.String("reply", reply),
		)
		if reply == "" {
			return nil
		}
		authCtx, err := h.authenticator.AuthorizedContext(ctx)
		if err != nil {
			return err
		}
		h.logger.Info(
			"post object",
			slog.Any("post", post.GetPost()),
		)
		communityID := post.GetPost().GetCommunityId()
		h.communityID = communityID
		replyTo := post.GetPost().GetPostId()

		if shouldReply {
			_, err = h.apiClient.CreatePost(
				authCtx,
				&application_apiv1.CreatePostRequest{
					CommunityId: &communityID,

					Text:            reply,
					InReplyToPostId: &replyTo,
				},
			)
		} else {
			_, err = h.apiClient.CreatePost(
				authCtx,
				&application_apiv1.CreatePostRequest{
					CommunityId: &communityID,
					Text:        reply,
				},
			)
		}
		if err != nil {
			h.logger.Error("failed to create community post",
				slog.String("error", err.Error()),
			)
			return err
		}

		h.logger.Info("community post created")
		stamps, err := h.apiClient.GetStamps(
			authCtx,
			&application_apiv1.GetStampsRequest{
				CommunityIds: []string{communityID},
			},
		)
		if err != nil {
			h.logger.Error("failed to get stamps",
				slog.String("error", err.Error()),
			)
			return nil
		}

		if len(stamps.CommunityStampSets) > 0 &&
			len(stamps.CommunityStampSets[0].Stamps) > 0 {

			set := stamps.CommunityStampSets[rand.Intn(len(stamps.CommunityStampSets))]
			stamp := set.Stamps[rand.Intn(len(set.Stamps))]

			_, err = h.apiClient.AddStampToPost(
				authCtx,
				&application_apiv1.AddStampToPostRequest{
					PostId:  replyTo,
					StampId: stamp.StampId,
				},
			)

			if err != nil {
				h.logger.Error("AddStampToPost failed",
					slog.String("error", err.Error()),
				)
			} else {
				h.logger.Info("AddStampToPost success")
			}
		}
	case constv1.EventType_EVENT_TYPE_CHAT_MESSAGE_RECEIVED:
		h.logger.Info("received CHAT_MESSAGE_RECEIVED event",
			slog.String("event_id", ev.EventId),
		)
		if err := h.handleChatMessage(ctx, ev.GetChatMessageReceivedEvent()); err != nil {
			h.logger.Error("failed to handle chat message", slog.String("error", err.Error()))
			return err
		}
	default:
		h.logger.Info("received event",
			slog.String("event_id", ev.EventId),
			slog.Int("event_type", int(ev.EventType)),
		)
	}
	return nil

}
func shouldReplyToPost(text string, isMention bool) bool {
	return true
}
func rememberKnowledge(text, displayName string) {

	var words []string
	displayName = strings.TrimSpace(displayName)

	if displayName != "" {
		text = strings.ReplaceAll(text, displayName, "")
	}
	skipKeys := map[string]bool{
		"今日": true,
		"昨日": true,
		"明日": true,
		"今年": true,
		"来年": true,
		"今":  true,
	}
	if strings.Contains(text, "さんは") &&
		strings.Contains(text, "だよ") {
		return
	}

	if strings.HasPrefix(text, "だせい、") &&
		strings.Contains(text, "は") &&
		strings.Contains(text, "だよ") {
		return
	}
	slog.Info("rememberKnowledge called")
	tokens := wakati.Tokenize(text)
	for _, token := range tokens {
		slog.Info("TOKEN",
			slog.String("surface", token.Surface),
			slog.Any("features", token.Features()),
		)
	}
	for _, token := range tokens {
		features := token.Features()

		if len(features) == 0 {
			continue
		}

		slog.Info("remember",
			slog.String("surface", token.Surface),
			slog.Any("feature", features),
		)
	}

	for _, token := range tokens {
		surface := strings.TrimSpace(token.Surface)

		if isProtectedName(surface) {
			continue
		}

		if surface == displayName {
			continue
		}
		skip := false

		for _, w := range fixedWords {
			if strings.Contains(w, surface) && w != surface {
				skip = true
				break
			}
		}

		if skip {
			continue
		}
		if surface == "" {
			continue
		}
		// 1文字は学習しない（絵文字・顔文字は除外）
		runes := []rune(surface)

		if len(runes) == 1 {
			isFixed := false

			for _, w := range fixedWords {
				if surface == w {
					isFixed = true
					break
				}
			}

			if !isFixed &&
				!unicode.IsSymbol(runes[0]) &&
				!unicode.IsPunct(runes[0]) {
				continue
			}
		}
		features := token.Features()

		if len(features) == 0 {
			continue
		}

		pos := features[0]

		switch pos {
		case "名詞", "形容詞", "副詞", "動詞", "助詞", "助動詞":
			words = append(words, surface)
		}
	}

	for i := 0; i < len(words)-1; i++ {

		if skipKeys[words[i]] || skipKeys[words[i+1]] {
			continue
		}

		learnedWordsMu.Lock()

		exists := false

		for _, pair := range learnedPairs {
			if pair.Key == words[i] && pair.Value == words[i+1] {
				exists = true
				break
			}
		}

		if !exists {
			learnedPairs = append(learnedPairs, LearnedPair{
				Key:   words[i],
				Value: words[i+1],
			})

			if err := SaveLearnedPair(words[i], words[i+1]); err != nil {
				slog.Error("SaveLearnedPair failed", slog.String("error", err.Error()))
			}
		}

		learnedWordsMu.Unlock()

		slog.Info("learned pair",
			slog.String("key", words[i]),
			slog.String("value", words[i+1]),
		)
	}
}
func isProtectedName(name string) bool {
	name = strings.TrimSpace(name)

	if name == "" {
		return false
	}

	// 現在登録されているコミュニティメンバーの表示名
	membersMu.RLock()
	for _, displayName := range members {
		if strings.Contains(name, displayName) {
			membersMu.RUnlock()
			return true
		}
	}
	membersMu.RUnlock()

	// ニックネーム機能で登録された名前・ニックネーム
	nicknameMu.RLock()
	for originalName, nickname := range nicknames {
		if name == originalName || name == nickname {
			nicknameMu.RUnlock()
			return true
		}
	}
	nicknameMu.RUnlock()

	return false
}
func isNicknameCommand(text string) bool {
	return strings.HasPrefix(text, "だせい、") &&
		strings.Contains(text, "さんは") &&
		strings.Contains(text, "だよ")
}
func searchWikipedia(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}

	searchURL := "https://ja.wikipedia.org/w/api.php?action=query&list=search&srsearch=" +
		url.QueryEscape(text) +
		"&format=json&utf8=1"

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		log.Printf("Wikipedia検索リクエスト作成エラー: %v", err)
		return ""
	}

	req.Header.Set("User-Agent", "Dasei/1.0 (mixi2 community plugin)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Wikipedia検索HTTPエラー: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var searchData struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return ""
	}

	if len(searchData.Query.Search) == 0 {
		return ""
	}

	title := searchData.Query.Search[0].Title

	extractURL := "https://ja.wikipedia.org/w/api.php?action=query&prop=extracts&exintro=1&explaintext=1&exchars=500&redirects=1&titles=" +
		url.QueryEscape(title) +
		"&format=json&utf8=1"

	req, err = http.NewRequest("GET", extractURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Dasei/1.0 (mixi2 community plugin)")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var extractData struct {
		Query struct {
			Pages map[string]struct {
				Title   string `json:"title"`
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&extractData); err != nil {
		return ""
	}

	for _, page := range extractData.Query.Pages {
		if page.Extract == "" {
			return ""
		}

		return page.Title + "：" + page.Extract
	}

	return ""
}
func searchWeb(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}

	searchURL := "https://www.bing.com/search?q=" +
		url.QueryEscape(text)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		log.Printf("Web検索リクエスト作成エラー: %v", err)
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Web検索HTTPエラー: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Web検索HTTPステータス: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Web検索本文読み込みエラー: %v", err)
		return ""
	}

	htmlText := string(body)
	log.Printf("Bing検索HTTP成功 status=%d bytes=%d query=%s", resp.StatusCode, len(body), text)
	titleRe := regexp.MustCompile(
		`<li class="b_algo"[^>]*>.*?<h2><a[^>]*>(.*?)</a>`,
	)

	snippetRe := regexp.MustCompile(
		`<li class="b_algo"[^>]*>.*?<p[^>]*>(.*?)</p>`,
	)
	titles := titleRe.FindAllStringSubmatch(htmlText, 5)
	snippets := snippetRe.FindAllStringSubmatch(htmlText, 5)
	stripHTML := func(s string) string {
		s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
		s = html.UnescapeString(s)
		return strings.TrimSpace(s)
	}

	var results []string

	for i := 0; i < len(titles); i++ {
		title := stripHTML(titles[i][1])
		snippet := ""

		if i < len(snippets) {
			snippet = stripHTML(snippets[i][1])
		}

		if title != "" {
			results = append(results, title+"："+snippet)
		}
	}

	if len(results) == 0 {
		log.Printf("Web検索結果0件 query=%s", text)
		return ""
	}

	result := strings.Join(results, "\n")
	log.Printf("Web検索成功 query=%s results=%d", text, len(results))

	return result
}
func getNetworkReference(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}

	refURL := "https://api.duckduckgo.com/?q=" +
		url.QueryEscape(text) +
		"&format=json&no_html=1&skip_disambig=1"

	req, err := http.NewRequest("GET", refURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Dasei/1.0 (mixi2 community plugin)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var data struct {
		AbstractText string `json:"AbstractText"`
		Answer       string `json:"Answer"`
		Definition   string `json:"Definition"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}

	if data.AbstractText != "" {
		return data.AbstractText
	}

	if data.Definition != "" {
		return data.Definition
	}

	if data.Answer != "" {
		return data.Answer
	}

	return ""
}
func isKaomojiPost(text string) bool {
	text = strings.TrimSpace(text)

	if text == "" {
		return false
	}

	// 顔文字でよく使われる記号が含まれているか
	kaomojiSymbols := []string{
		"(*",
		"*)",
		"＼(^",
		"／",
		"＼",
		"ヾ",
		"ﾟ",
		"ω",
		"＾",
		"^",
		"三",
	}

	for _, symbol := range kaomojiSymbols {
		if strings.Contains(text, symbol) {
			return true
		}
	}

	return false
}
func detectIntent(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return "empty"
	}

	// 質問
	if strings.Contains(text, "？") ||
		strings.Contains(text, "?") {
		return "question"
	}

	// 感謝
	if strings.Contains(text, "ありがとう") ||
		strings.Contains(text, "ありがと") {
		return "thanks"
	}

	// 挨拶
	if strings.Contains(text, "おはよう") ||
		strings.Contains(text, "こんにちは") ||
		strings.Contains(text, "こんばんは") {
		return "greeting"
	}

	// 悲しみ・落ち込み
	if strings.Contains(text, "悲しい") ||
		strings.Contains(text, "かなしい") ||
		strings.Contains(text, "つらい") ||
		strings.Contains(text, "辛い") ||
		strings.Contains(text, "落ち込") {
		return "sad"
	}

	// 怒り・不満
	if strings.Contains(text, "ムカつ") ||
		strings.Contains(text, "むかつ") ||
		strings.Contains(text, "腹立") ||
		strings.Contains(text, "イライラ") ||
		strings.Contains(text, "最悪") {
		return "angry"
	}

	// 疲れ・体調
	if strings.Contains(text, "疲れ") ||
		strings.Contains(text, "しんど") ||
		strings.Contains(text, "眠い") ||
		strings.Contains(text, "眠た") ||
		strings.Contains(text, "寝不足") ||
		strings.Contains(text, "腰が痛") {
		return "condition"
	}

	// 食事
	if strings.Contains(text, "食べ") ||
		strings.Contains(text, "ご飯") ||
		strings.Contains(text, "ごはん") ||
		strings.Contains(text, "料理") ||
		strings.Contains(text, "ラーメン") ||
		strings.Contains(text, "寿司") ||
		strings.Contains(text, "焼肉") {
		return "food"
	}
	// 買い物
	if strings.Contains(text, "買おう") ||
		strings.Contains(text, "買いたい") ||
		strings.Contains(text, "欲しい") ||
		strings.Contains(text, "ほしい") ||
		strings.Contains(text, "買って") {
		return "shopping"
	}

	// 仕事・学校
	if strings.Contains(text, "仕事") ||
		strings.Contains(text, "会社") ||
		strings.Contains(text, "学校") ||
		strings.Contains(text, "出勤") ||
		strings.Contains(text, "退勤") {
		return "work"
	}

	// 旅行・外出
	if strings.Contains(text, "旅行") ||
		strings.Contains(text, "お出かけ") ||
		strings.Contains(text, "出かけ") ||
		strings.Contains(text, "遊びに") {
		return "outing"
	}

	// 天気・気温
	if strings.Contains(text, "暑い") ||
		strings.Contains(text, "暑かった") ||
		strings.Contains(text, "暑く") {
		return "hot"
	}

	if strings.Contains(text, "寒い") ||
		strings.Contains(text, "寒かった") ||
		strings.Contains(text, "寒く") {
		return "cold"
	}

	// 喜び
	if strings.Contains(text, "嬉しい") ||
		strings.Contains(text, "うれしい") ||
		strings.Contains(text, "楽しい") ||
		strings.Contains(text, "楽しかった") {
		return "happy"
	}

	// 達成
	if strings.Contains(text, "できた") ||
		strings.Contains(text, "成功") ||
		strings.Contains(text, "完成") ||
		strings.Contains(text, "終わった") {
		return "achievement"
	}

	return "other"
}
func generateNaturalReply(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}
	// 顔文字・記号っぽい投稿
	if isKaomojiPost(text) {
		return randomReply(
			"わーい",
			"楽しそうだね",
			"なんか嬉しそうだね",
			"お、元気だね",
		)
	}
	// 質問への返答
	if strings.Contains(text, "どっちが好き") {
		return randomReply(
			"どっちも好きだよ",
			"うーん、迷うね",
			"どっちもいいね",
		)
	}

	if strings.Contains(text, "好き？") ||
		strings.Contains(text, "好きですか") ||
		strings.Contains(text, "好きなの？") {

		return randomReply(
			"好きだよ",
			"けっこう好きだよ",
			"うーん、どうだろう",
			"気分によるかな",
		)
	}
	intent := detectIntent(text)

	slog.Info("detected intent",
		slog.String("intent", intent),
		slog.String("text", text),
	)
	if intent == "hot" {
		return randomReply(
			"暑いね、今日も夏って感じだね",
			"暑いね、水分補給しないとね",
			"今日も暑そうだね",
			"これは暑さにやられるね",
		)
	}
	// 感謝・お礼
	if strings.Contains(text, "ありがとう") ||
		strings.Contains(text, "ありがと") {
		return randomReply(
			"どういたしまして",
			"いえいえ",
			"こちらこそだよ",
			"気にしないで",
		)
	}

	// 挨拶
	if strings.Contains(text, "おはよう") {
		return randomReply(
			"おはよう",
			"おはよう、今日もよろしく",
			"おはよう、いい朝だね",
		)
	}

	if strings.Contains(text, "こんにちは") {
		return randomReply(
			"こんにちは",
			"こんにちは、今日もよろしく",
			"こんにちは、元気そうだね",
		)
	}

	if strings.Contains(text, "こんばんは") {
		return randomReply(
			"こんばんは",
			"こんばんは、ゆっくりしてね",
			"こんばんは、今日もおつかれさま",
		)
	}

	// 疲れ・体調
	if strings.Contains(text, "疲れた") ||
		strings.Contains(text, "疲れちゃった") ||
		strings.Contains(text, "しんどい") ||
		strings.Contains(text, "しんどかった") {
		return randomReply(
			"それはおつかれさま",
			"大変だったね、ゆっくり休んでね",
			"今日はゆっくりしたほうがよさそうだね",
			"おつかれさま、無理しないでね",
		)
	}

	if strings.Contains(text, "眠い") ||
		strings.Contains(text, "眠た") ||
		strings.Contains(text, "寝不足") {
		return randomReply(
			"眠いときは無理しないでね",
			"今日は早めに休めるといいね",
			"それは眠くなるね",
			"ゆっくり休んでね",
		)
	}

	// 悲しみ・落ち込み
	if strings.Contains(text, "悲しい") ||
		strings.Contains(text, "かなしい") ||
		strings.Contains(text, "つらい") ||
		strings.Contains(text, "辛い") ||
		strings.Contains(text, "落ち込") {
		return randomReply(
			"よしよしヾ(・ω・｀)",
			"無理しなくていいと思うよ",
			"そういう日もあるよ",
			"ゆっくりでいいと思うよ",
		)
	}
	// 怒り・不満
	if strings.Contains(text, "ムカつ") ||
		strings.Contains(text, "むかつ") ||
		strings.Contains(text, "腹立") ||
		strings.Contains(text, "イライラ") ||
		strings.Contains(text, "最悪") {
		return randomReply(
			"アカンなぁ",
			"どんまいだよ",
			"(# ﾟДﾟ)",
			"それはヤダね",
		)
	}

	// 喜び・達成
	if strings.Contains(text, "嬉しい") ||
		strings.Contains(text, "うれしい") ||
		strings.Contains(text, "楽しい") ||
		strings.Contains(text, "楽しかった") {
		return randomReply(
			"ええやん",
			"いいね、楽しそう",
			"それは嬉しいね",
			"ステキやん！",
		)
	}

	if strings.Contains(text, "できた") ||
		strings.Contains(text, "成功") ||
		strings.Contains(text, "完成") ||
		strings.Contains(text, "終わった") {
		return randomReply(
			"おお、よかった！",
			"最高",
			"ええやん！",
			"おつかれさま！やったね！",
		)
	}

	// 食事・食べ物
	if strings.Contains(text, "美味しくない") ||
		strings.Contains(text, "美味しくなかった") ||
		strings.Contains(text, "まずい") ||
		strings.Contains(text, "不味い") {
		return randomReply(
			"それは残念だね",
			"せっかくなのにねぇ",
			"どんまい",
		)
	}

	if strings.Contains(text, "美味しい") ||
		strings.Contains(text, "おいしい") ||
		strings.Contains(text, "うまい") ||
		strings.Contains(text, "美味しかった") {
		return randomReply(
			"それはよすぎる",
			"美味しいものは幸せだね",
			"ええもん食うてるやん",
			"お腹すいてきた…",
		)
	}
	// 食べ物についての質問
	if strings.Contains(text, "どっち") ||
		strings.Contains(text, "どちら") ||
		strings.Contains(text, "好き？") ||
		strings.Contains(text, "好きー？") ||
		strings.Contains(text, "苦手？") ||
		strings.Contains(text, "嫌い？") {

		return randomReply(
			"だせいはご飯派かな",
			"どっちも好きだよ",
			"うーん、迷うね",
			"気分によるかな",
		)
	}
	if strings.Contains(text, "食べた") ||
		strings.Contains(text, "食べてきた") ||
		strings.Contains(text, "食べる") ||
		strings.Contains(text, "ご飯") ||
		strings.Contains(text, "ごはん") ||
		strings.Contains(text, "料理") ||
		strings.Contains(text, "ラーメン") ||
		strings.Contains(text, "寿司") ||
		strings.Contains(text, "焼肉") {
		return randomReply(
			"よすぎる",
			"うらやましい！",
			"えー！いいな！",
			"それ聞いたらお腹すいてきた…",
			"ええもん食うてるな自分",
		)
	}
	// 買い物・欲しいもの
	if strings.Contains(text, "買おう") ||
		strings.Contains(text, "買いたい") ||
		strings.Contains(text, "欲しい") ||
		strings.Contains(text, "ほしい") ||
		strings.Contains(text, "買って") {
		return randomReply(
			"買っちゃえ！",
			"ね！気になる！",
			"悩むよねー(´・ω・`)",
			"いつかね",
		)
	}

	// 仕事・学校
	if strings.Contains(text, "仕事") ||
		strings.Contains(text, "会社") ||
		strings.Contains(text, "学校") ||
		strings.Contains(text, "出勤") ||
		strings.Contains(text, "退勤") {
		if strings.Contains(text, "終わ") ||
			strings.Contains(text, "帰") {
			return randomReply(
				"おきばりやす",
				"えらい！！",
				"帰ったらゆっくりしよ！",
			)
		}

		return randomReply(
			"今日もおつかれさま",
			"無理しないでね",
			"大変そうだね",
			"頑張ってるね",
		)
	}

	// 予定・旅行・お出かけ
	if strings.Contains(text, "旅行") ||
		strings.Contains(text, "お出かけ") ||
		strings.Contains(text, "出かけ") ||
		strings.Contains(text, "遊びに") {
		return randomReply(
			"ええやん！",
			"GO！GO!",
			"たのしみ！",
			"気をつけてね！",
		)
	}
	// 天気・気温
	if strings.Contains(text, "暑い") ||
		strings.Contains(text, "暑かった") {
		return randomReply(
			"西川貴教のせい",
			"松岡修造のせい",
			"水分補給忘れずにね",
		)
	}

	if strings.Contains(text, "寒い") ||
		strings.Contains(text, "寒かった") {
		return randomReply(
			"西川貴教のせい",
			"寒いよね((((；ﾟДﾟ))))",
			"あったかくしてね",
		)
	}

	// 質問
	if strings.Contains(text, "？") ||
		strings.Contains(text, "?") {

		// 好みを聞かれた
		if strings.Contains(text, "好き") ||
			strings.Contains(text, "好み") {
			return randomReply(
				"だせいはご飯派かな",
				"どっちも好きだよ",
				"その日の気分かな",
				"だせいはそっちが好きかも",
			)
		}

		// どちらかを選ぶ質問
		if strings.Contains(text, "どっち") ||
			strings.Contains(text, "どちら") {
			return randomReply(
				"だせいはこっちかな",
				"うーん、迷うね",
				"どっちもいいな",
				"その日の気分かな",
			)
		}

		// 理由を聞かれた
		if strings.Contains(text, "なんで") ||
			strings.Contains(text, "なぜ") ||
			strings.Contains(text, "どうして") {
			return randomReply(
				"なんとなくだよ",
				"そういう気分だったのかも",
				"だせいにもよくわからない",
				"なんでやろね",
			)
		}

		// 意見を聞かれた
		if strings.Contains(text, "どう思う") ||
			strings.Contains(text, "どう思います") ||
			strings.Contains(text, "どうかな") {
			return randomReply(
				"いいと思うよ",
				"だせいはアリだと思う",
				"それもいいんじゃないかな",
				"だせいは好きかも",
			)
		}

		// その他の質問
		return randomReply(
			"どうなんやろ",
			"だせいはそう思うかな",
			"気になるところだね",
			"うーん、難しいね",
		)
	}

	// 基本的な肯定
	if strings.Contains(text, "最高") ||
		strings.Contains(text, "良かった") ||
		strings.Contains(text, "よかった") {
		return randomReply(
			"それはよかったね",
			"いいね",
			"それは嬉しいね",
			"やったー！",
		)
	}

	// 最終フォールバック
	return randomReply(
		"そうなんだね",
		"なるほど",
		"そういうことか",
		"いいね",
		"それは気になるね",
	)
}
func generateReplyWithWebCheck(text string) string {
	// まず、現在のだせいの返事を作る
	reply := generateNaturalReply(text)

	if reply == "" {
		return ""
	}

	// Wikipedia検索
	webResult := searchWikipedia(text)

	if webResult != "" {
		slog.Info("Wikipedia result",
			slog.String("text", text),
			slog.String("result", webResult),
		)

	}

	return reply
}

// CreateReplyWithWebCheck は外部から呼び出すための返信生成入口
func CreateReplyWithWebCheck(text string) string {
	return generateReplyWithWebCheck(text)
}
func polishDaseiReply(originalText string, reply string) string {
	originalText = strings.TrimSpace(originalText)
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	wikipediaResult := searchWikipedia(originalText)
	networkResult := searchWeb(originalText)

	webResult := wikipediaResult + "\n" + networkResult

	slog.Info("Reference search",
		slog.String("original", originalText),
		slog.String("reply", reply),
		slog.String("wikipedia", wikipediaResult),
		slog.String("network", networkResult),
	)

	referenceWords := extractReferenceWords(webResult)
	referenceSentences := extractReferenceSentences(webResult)

	reply = polishWithReference(reply, originalText, referenceWords, referenceSentences)
	reply = checkReferenceSentences(reply, referenceSentences)

	return normalizeDaseiReply(reply)
}
func checkReferenceSentences(reply string, referenceSentences []string) string {
	if reply == "" || len(referenceSentences) == 0 {
		return reply
	}

	replyTokens := tokenizeForPolish(reply)

	if len(replyTokens) == 0 {
		return reply
	}

	for _, sentence := range referenceSentences {
		referenceTokens := tokenizeForPolish(sentence)

		if len(referenceTokens) == 0 {
			continue
		}

		matched := countCommonWords(replyTokens, referenceTokens)

		if matched > 0 {
			slog.Info("Wikipedia filter sentence matched",
				slog.String("sentence", sentence),
				slog.Int("matched", matched),
			)
		}
	}

	return reply
}
func isFactCheckReply(originalText string, reply string) bool {
	originalText = strings.TrimSpace(originalText)
	reply = strings.TrimSpace(reply)

	if originalText == "" || reply == "" {
		return false
	}

	// 質問形式でなければ校正対象外
	if !strings.Contains(originalText, "？") &&
		!strings.Contains(originalText, "?") {
		return false
	}

	// 意見・好みなどの質問は校正対象外
	if strings.Contains(originalText, "好き") ||
		strings.Contains(originalText, "どっち") ||
		strings.Contains(originalText, "どちら") ||
		strings.Contains(originalText, "どう思う") ||
		strings.Contains(originalText, "どうかな") {
		return false
	}

	// 事実確認につながりやすい質問
	keywords := []string{
		"いつ",
		"どこ",
		"誰",
		"何",
		"なに",
		"何年",
		"何月",
		"何日",
		"何歳",
		"いくら",
		"何人",
		"どんな",
		"とは",
		"について",
		"意味",
		"由来",
		"歴史",
		"場所",
		"名前",
		"日付",
		"ニュース",
	}

	for _, keyword := range keywords {
		if strings.Contains(originalText, keyword) {
			return true
		}
	}

	return false
}
func tokenizeForPolish(text string) []string {
	text = strings.TrimSpace(text)

	if text == "" {
		return nil
	}

	tokens := wakati.Tokenize(text)

	var words []string

	for _, token := range tokens {
		surface := strings.TrimSpace(token.Surface)

		if surface == "" {
			continue
		}

		// 記号は除外
		if isPolishSymbol(surface) {
			continue
		}

		words = append(words, surface)
	}

	return words
}
func countCommonWords(replyTokens []string, referenceTokens []string) int {
	if len(replyTokens) == 0 || len(referenceTokens) == 0 {
		return 0
	}

	referenceSet := make(map[string]bool)

	for _, word := range referenceTokens {
		referenceSet[word] = true
	}

	matched := 0
	already := make(map[string]bool)

	for _, word := range replyTokens {
		if already[word] {
			continue
		}

		// BOS / EOS は照合対象外
		if word == "BOS" || word == "EOS" {
			continue
		}

		if referenceSet[word] {
			matched++
			already[word] = true

			slog.Info("Wikipedia filter word matched",
				slog.String("word", word),
			)
		}
	}

	return matched
}
func isPolishSymbol(text string) bool {
	if text == "" {
		return true
	}

	for _, r := range text {
		switch r {
		case '。', '、', '！', '？',
			'「', '」', '『', '』',
			'（', '）', '(', ')',
			'・', '…', 'ー',
			',', '.', '!', '?':
			continue
		default:
			return false
		}
	}

	return true
}
func extractReferenceWords(reference string) []string {
	reference = strings.TrimSpace(reference)

	if reference == "" {
		return nil
	}

	// Wikipediaのタイトル部分と本文を分離
	parts := strings.SplitN(reference, "：", 2)

	var text string
	if len(parts) == 2 {
		text = parts[0] + " " + parts[1]
	} else {
		text = reference
	}

	// Kagomeで日本語を単語単位に分解
	tokens := wakati.Tokenize(text)

	var words []string

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" {
			continue
		}

		if word == "BOS" || word == "EOS" {
			continue
		}

		// 記号は除外
		if isPolishSymbol(word) {
			continue
		}

		// 1文字だけの語は除外
		if len([]rune(word)) < 2 {
			continue
		}

		words = append(words, word)
	}

	return words
}
func extractReferenceSentences(reference string) []string {
	reference = strings.TrimSpace(reference)

	if reference == "" {
		return nil
	}

	parts := strings.SplitN(reference, "：", 2)

	var text string
	if len(parts) == 2 {
		text = parts[1]
	} else {
		text = reference
	}

	// Wikipedia本文を「実際の文章」の単位で分ける
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '。', '！', '？':
			return true
		default:
			return false
		}
	})

	var result []string

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)

		if len([]rune(sentence)) < 4 {
			continue
		}

		result = append(result, sentence)
	}

	return result
}
func expandShortDaseiReply(reply string, originalText string, referenceWords []string, referenceSentences []string) string {
	reply = strings.TrimSpace(reply)
	originalText = strings.TrimSpace(originalText)

	if reply == "" {
		return reply
	}

	// 50文字以上なら、そのまま返す
	if len([]rune(reply)) >= 50 {
		return reply
	}

	// 返信を自然に補強するための材料を探す
	contextWord := ""

	for _, word := range referenceWords {
		word = strings.TrimSpace(word)

		if word == "" || len([]rune(word)) < 2 {
			continue
		}

		if strings.Contains(originalText, word) {
			contextWord = word
			break
		}
	}

	// 検索結果から使えそうな短い文章を探す
	contextSentence := ""

	for _, sentence := range referenceSentences {
		sentence = strings.TrimSpace(sentence)

		if sentence == "" {
			continue
		}

		runes := []rune(sentence)

		if len(runes) > 60 {
			sentence = string(runes[:60])
		}

		contextSentence = sentence
		break
	}
	// 具体的な単語が取れている場合
	if contextWord != "" {
		addition := contextWord + "の話やね。投稿を見てると、こういうところが気になるところやね。だせいももう少し詳しく知りたいかな。"

		remaining := 149 - len([]rune(reply))
		if remaining > 0 {
			addRunes := []rune(addition)

			if len(addRunes) > remaining {
				addition = string(addRunes[:remaining])
			}

			reply += addition
		}
	}

	// まだ50文字未満なら、検索結果の文章を利用
	if len([]rune(reply)) < 50 && contextSentence != "" {
		addition := "調べてみると、" + contextSentence + "という話もあるみたい。投稿の内容と合わせて考えると、ちょっと気になるところやね。"

		remaining := 149 - len([]rune(reply))
		if remaining > 0 {
			addRunes := []rune(addition)

			if len(addRunes) > remaining {
				addition = string(addRunes[:remaining])
			}

			reply += addition
		}
	}

	// 検索結果が使えない場合でも、自然な補足を行う
	if len([]rune(reply)) < 50 {
		addition := " 投稿の内容を見てると、そういうことについて考えてみるのも面白そうやね。だせいも気になるところかな。"

		remaining := 149 - len([]rune(reply))
		if remaining > 0 {
			addRunes := []rune(addition)

			if len(addRunes) > remaining {
				addition = string(addRunes[:remaining])
			}

			reply += addition
		}
	}

	// 最終的に149文字を超えないようにする
	runes := []rune(reply)

	if len(runes) > 149 {
		reply = string(runes[:149])
	}

	return strings.TrimSpace(reply)
}
func polishWithReference(reply string, originalText string, referenceWords []string, referenceSentences []string) string {
	if reply == "" {
		return reply
	}

	originalTokens := tokenizeForPolish(originalText)

	if len(originalTokens) == 0 {
		return reply
	}

	// 検索結果から、元の返信と最も関連性の高い文章を探す
	bestSentence := ""
	bestScore := 0

	for _, sentence := range referenceSentences {
		sentence = strings.TrimSpace(sentence)

		if sentence == "" {
			continue
		}

		sentenceTokens := tokenizeForPolish(sentence)

		if len(sentenceTokens) == 0 {
			continue
		}

		score := countRelevantCommonWords(originalTokens, sentenceTokens)

		if score > bestScore {
			bestScore = score
			bestSentence = sentence
		}
	}

	// 関連する検索結果がなければ、元の返信をそのまま返す
	if bestSentence == "" || bestScore < 2 {
		slog.Info("Wikipedia polish skipped",
			slog.String("reply", reply),
			slog.Int("score", bestScore),
		)
		return reply
	}

	// 検索結果との関連性を記録
	slog.Info("Wikipedia polish reference selected",
		slog.String("reply", reply),
		slog.Int("score", bestScore),
		slog.String("reference", bestSentence),
	)
	usefulWords := extractUsefulReferenceWords(referenceWords)

	slog.Info("Wikipedia useful reference words",
		slog.String("reply", reply),
		slog.Int("count", len(usefulWords)),
		slog.Any("words", usefulWords),
	)
	reply = applyReferenceWords(reply, usefulWords)
	enrichedReply := enrichDaseiReply(
		reply,
		originalText,
		usefulWords,
		referenceSentences,
	)

	if enrichedReply != reply {
		slog.Info("Wikipedia reply enriched",
			slog.String("before", reply),
			slog.String("after", enrichedReply),
		)

		reply = enrichedReply
	}
	// 短文の場合は、元の投稿と検索結果を参考にして
	// 50〜149文字程度の自然な返信へ膨らませる。
	//
	// 50文字以上の返信は変更しない。
	// 149文字を超えている返信も無理に削らない。
	reply = ensureDaseiReplyLength(
		reply,
		originalText,
		usefulWords,
		referenceSentences,
	)

	// 最後に空白・句読点などを整理する。
	return normalizeDaseiReply(reply)
}

func countRelevantCommonWords(a []string, b []string) int {
	stopWords := map[string]bool{
		"は":  true,
		"が":  true,
		"を":  true,
		"に":  true,
		"へ":  true,
		"と":  true,
		"の":  true,
		"で":  true,
		"も":  true,
		"や":  true,
		"から": true,
		"まで": true,
		"だけ": true,
		"など": true,
		"こと": true,
		"もの": true,
		"これ": true,
		"それ": true,
		"あれ": true,
		"この": true,
		"その": true,
		"あの": true,
		"何":  true,
		"日":  true,
		"今日": true,
	}

	count := 0

	for _, wordA := range a {
		wordA = strings.TrimSpace(wordA)

		if wordA == "" || stopWords[wordA] {
			continue
		}

		if len([]rune(wordA)) < 2 {
			continue
		}

		for _, wordB := range b {
			wordB = strings.TrimSpace(wordB)

			if wordB == "" || stopWords[wordB] {
				continue
			}

			if wordA == wordB {
				count++
				break
			}
		}
	}

	return count
}
func extractUsefulReferenceWords(referenceWords []string) []string {
	if len(referenceWords) == 0 {
		return nil
	}

	var result []string

	for _, word := range referenceWords {
		word = strings.TrimSpace(word)

		if word == "" {
			continue
		}

		// 短すぎる語は除外
		if len([]rune(word)) < 2 {
			continue
		}

		// 一般的すぎる語は除外
		switch word {
		case "今日", "現在", "地域", "場所",
			"情報", "ニュース", "記事", "内容",
			"人物", "名称", "東京都", "日本":
			continue
		}

		// 重複除去
		duplicate := false

		for _, existing := range result {
			if existing == word {
				duplicate = true
				break
			}
		}

		if duplicate {
			continue
		}

		result = append(result, word)

		// 使う候補は最大5個
		if len(result) >= 5 {
			break
		}
	}

	return result
}
func applyReferenceWords(reply string, usefulWords []string) string {
	if reply == "" || len(usefulWords) == 0 {
		return reply
	}

	replyTokens := tokenizeForPolish(reply)

	if len(replyTokens) == 0 {
		return reply
	}

	matched := 0

	for _, word := range usefulWords {
		word = strings.TrimSpace(word)

		if word == "" {
			continue
		}

		for _, token := range replyTokens {
			if token == word {
				matched++
				break
			}
		}
	}

	slog.Info("Wikipedia reference words applied",
		slog.String("reply", reply),
		slog.Int("matched", matched),
		slog.Int("usefulWords", len(usefulWords)),
	)

	return reply
}
func enrichDaseiReply(reply string, originalText string, referenceWords []string, referenceSentences []string) string {
	reply = strings.TrimSpace(reply)
	originalText = strings.TrimSpace(originalText)

	if reply == "" || originalText == "" {
		return reply
	}

	isGenericReply := false

	switch reply {
	case "どうなんやろ",
		"気になるところだね",
		"そうなんだね",
		"なるほど",
		"そういうことか",
		"いいね":
		isGenericReply = true
	}

	if !isGenericReply {
		return reply
	}

	for _, word := range referenceWords {
		word = strings.TrimSpace(word)

		if word == "" {
			continue
		}

		if len([]rune(word)) < 2 {
			continue
		}

		if !strings.Contains(originalText, word) {
			continue
		}

		if strings.Contains(reply, word) {
			continue
		}

		switch reply {
		case "どうなんやろ":
			return word + "の話なんやろね"

		case "気になるところだね":
			return word + "の話、気になるね"

		case "そうなんだね":
			return word + "なんやね"

		case "なるほど":
			return word + "なんやね"

		case "そういうことか":
			return word + "の話なんやね"

		case "いいね":
			return word + "なんやね、いいね"
		}
	}
	// 検索結果に具体的な文章があり、
	// 元投稿が事実確認型なら、検索結果を使って具体化する
	if len(referenceSentences) > 0 && isFactCheckReply(originalText, reply) {
		sentence := strings.TrimSpace(referenceSentences[0])

		if sentence != "" {
			// 検索結果そのものを長文で返さない
			runes := []rune(sentence)

			if len(runes) > 80 {
				sentence = string(runes[:80]) + "…"
			}

			switch reply {
			case "どうなんやろ",
				"気になるところだね",
				"そうなんだね",
				"なるほど",
				"そういうことか",
				"いいね":
				return "調べてみたら、" + sentence + " みたいだね"
			}
		}
	}

	return reply
}
func normalizeDaseiPostLength(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}

	// すでに50〜149文字ならそのまま
	length := len([]rune(text))
	if length >= 50 && length <= 149 {
		return text
	}

	// 50文字未満なら、だせいらしい自然な一言を追加
	additions := []string{
		"なんとなくそんな気分だよ",
		"こういうことってあるよね",
		"だせいはそう思ったりするよ",
		"まあ、そういう日もあるよね",
		"特に理由はないけどそんな感じだよ",
		"今日はなんとなくそういう気分かな",
		"だせいもよく分からないけどね",
	}

	for length < 50 {
		addition := additions[rand.Intn(len(additions))]
		text += addition
		length = len([]rune(text))
	}

	// 149文字を超えた場合は149文字で切る
	runes := []rune(text)
	if len(runes) > 149 {
		text = string(runes[:149])
	}

	return text
}
func normalizeDaseiReply(reply string) string {
	slog.Info("Dasei normalize CALLED", slog.String("reply", reply))
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	// 連続する空白を整理
	reply = strings.Join(strings.Fields(reply), " ")

	// だせいの追加文が前の文章にくっついた場合、
	// 文の区切りを入れて読みやすくする。
	separators := []string{
		"だせいは",
		"だせいも",
		"だせいが",
		"だせいには",
		"だせいにも",
	}

	for _, separator := range separators {
		for {
			index := strings.Index(reply, separator)

			// 文頭ならそのまま
			if index <= 0 {
				break
			}

			// すでに区切られているなら何もしない
			beforeText := reply[:index]

			if strings.HasSuffix(beforeText, "。") ||
				strings.HasSuffix(beforeText, "！") ||
				strings.HasSuffix(beforeText, "？") ||
				strings.HasSuffix(beforeText, "、") ||
				strings.HasSuffix(beforeText, "\n") {
				break
			}

			// 「だせい」の前に句点を入れる
			reply = reply[:index] + "。" + reply[index:]

			break
		}
	}

	// 連続する句読点を整理
	for {
		old := reply

		reply = strings.ReplaceAll(reply, "。。", "。")
		reply = strings.ReplaceAll(reply, "！！", "！")
		reply = strings.ReplaceAll(reply, "？？", "？")
		reply = strings.ReplaceAll(reply, "、、", "、")

		if reply == old {
			break
		}
	}

	reply = addNaturalDaseiPunctuation(reply)

	return reply
}
func addNaturalDaseiPunctuation(reply string) string {
	slog.Info("PUNCTUATION BEFORE", slog.String("reply", reply))
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return reply
	}

	sentencePairs := []struct {
		ending  string
		starter string
	}{
		{"だろう", "特に"},
		{"だろう", "なんとなく"},
		{"だろう", "まあ"},
		{"だよ", "だせい"},
		{"だよ", "まあ"},
		{"だよ", "なんとなく"},
		{"だよ", "そういう"},
		{"だよ", "こういう"},
		{"するよ", "まあ"},
		{"するよ", "なんとなく"},
		{"よね", "なんとなく"},
		{"よね", "まあ"},
		{"ですよ", "まあ"},
		{"ですね", "まあ"},
	}
	for _, pair := range sentencePairs {
		from := pair.ending + pair.starter
		to := pair.ending + "。" + pair.starter
		reply = strings.Replace(reply, from, to, 1)
	}

	commaWords := []string{
		"けれど",
		"けど",
		"ので",
	}

	for _, word := range commaWords {
		index := strings.Index(reply, word)

		if index < 0 {
			continue
		}

		end := index + len(word)

		if end >= len(reply) {
			continue
		}

		next := reply[end:]

		if !strings.HasPrefix(next, "。") &&
			!strings.HasPrefix(next, "、") &&
			!strings.HasPrefix(next, "！") &&
			!strings.HasPrefix(next, "？") {
			reply = reply[:end] + "、" + reply[end:]
		}
	}
	slog.Info("PUNCTUATION AFTER", slog.String("reply", reply))
	return reply
}
func ensureDaseiReplyLength(reply string, originalText string, referenceWords []string, referenceSentences []string) string {
	reply = strings.TrimSpace(reply)
	originalText = strings.TrimSpace(originalText)

	if reply == "" {
		return reply
	}

	// 短文でなければ、そのまま返す
	runeCount := len([]rune(reply))

	if runeCount >= 50 {
		return reply
	}

	// 元の投稿に含まれる具体的な単語を探す
	usefulWord := ""

	for _, word := range referenceWords {
		word = strings.TrimSpace(word)

		if word == "" {
			continue
		}

		if len([]rune(word)) < 2 {
			continue
		}

		if strings.Contains(originalText, word) {
			usefulWord = word
			break
		}
	}

	// 返信を自然に膨らませる
	candidates := []string{}

	if usefulWord != "" {
		candidates = append(candidates,
			reply+"。"+usefulWord+"の話として見ると、ちょっと気になるところだね。",
			reply+"。"+usefulWord+"については、そういう見方もありそうやね。だせいもちょっと気になる。",
			reply+"。"+usefulWord+"のことを考えると、もう少し詳しく見てみたいところやね。",
		)
	}

	if originalText != "" {
		candidates = append(candidates,
			reply+"。元の話を見る限り、もう少し詳しいところまで知りたくなる内容やね。",
			reply+"。この話だけではまだ何とも言えんけど、ちょっと気になるところではあるね。",
		)
	}

	if len(candidates) == 0 {
		return reply
	}

	// 50〜149文字に収まる候補を優先
	for _, candidate := range candidates {

		count := len([]rune(candidate))

		if count >= 50 && count <= 149 {
			slog.Info("Dasei short reply expanded",
				slog.String("before", reply),
				slog.String("after", candidate),
				slog.Int("length", count),
			)

			return candidate
		}
	}

	return reply
}

func createReply(text string) string {

	// 名前を呼ばれたら必ず返信
	if strings.Contains(text, "だせい") || strings.Contains(text, "惰性") {
		slog.Info("force reply")

		// 返信しない判定はスキップ
	}

	if strings.HasPrefix(text, "だせい、") &&
		strings.Contains(text, "さんは") &&
		strings.Contains(text, "だよ") {

		body := strings.TrimPrefix(text, "だせい、")

		parts := strings.SplitN(body, "さんは", 2)
		if len(parts) == 2 {

			name := strings.TrimSpace(parts[0])

			nickname := strings.TrimSuffix(parts[1], "だよ")
			nickname = strings.TrimSpace(nickname)
			nickname = strings.TrimSuffix(nickname, "さん")
			nickname = strings.TrimSpace(nickname)
			nicknameMu.Lock()
			nicknames[name] = nickname
			nicknameMu.Unlock()

			teachMu.Lock()
			delete(teaches, name)
			teachMu.Unlock()
			learnedWordsMu.Lock()

			var filteredPairs []LearnedPair
			for _, pair := range learnedPairs {
				if pair.Key == name || pair.Value == name {
					continue
				}
				filteredPairs = append(filteredPairs, pair)
			}
			learnedPairs = filteredPairs

			var filteredPhrases []LearnedPhrase
			for _, phrase := range learnedPhrases {
				if strings.Contains(phrase.Text, name) {
					continue
				}
				filteredPhrases = append(filteredPhrases, phrase)
			}
			learnedPhrases = filteredPhrases

			learnedWordsMu.Unlock()
			return addEmoji("わかった！")
		}
	}

	slog.Info("received text",
		slog.String("text", text),
	)
	if strings.HasPrefix(text, "だせい、") && strings.Contains(text, "は") && strings.Contains(text, "だよ") {
		slog.Info("teach pattern detected")
		body := strings.TrimPrefix(text, "だせい、")

		parts := strings.SplitN(body, "は", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSuffix(parts[1], "だよ")
			value = strings.TrimSpace(value)

			if isProtectedName(key) || isProtectedName(value) {
				return addEmoji("それは名前として覚えないよ")
			}

			memoryMu.Lock()
			memories[key] = Memory{
				Value:     value,
				LearnedAt: time.Now(),
			}
			memoryMu.Unlock()

			if err := SaveMemory(key, value); err != nil {
				slog.Error("SaveMemory failed",
					slog.String("error", err.Error()),
				)
			} else {
				slog.Info("SaveMemory success",
					slog.String("key", key),
					slog.String("value", value),
				)
			}

			return addEmoji("わかった！")
		}
	} // ← これを追加
	text = applyNicknames(text)
	reply := replyFromMemory(text)
	if reply != "" {
		return reply
	}

	// 返信の10%だけインプレゾンビ
	// 返信の10%だけインプレゾンビ
	if rand.Intn(100) < 10 {

		mode := rand.Intn(100)

		switch {

		case mode < 30:
			return zombieEnglish()

		case mode < 60:
			return zombieJapanese()

		default:
			return zombieReply(text)
		}
	}
	// 自然な日本語で返信
	reply = generateNaturalReply(text)

	if reply != "" {
		return polishDaseiReply(text, reply)
	}

	return "そうなんだね"
}
func nicknameOf(name string) string {
	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	if nickname, ok := nicknames[name]; ok {
		return nickname
	}

	return ""
}
func memberID(name string) string {
	return name
}
func shapeDaseiParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	var result strings.Builder

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		result.WriteString(part)
	}

	text := strings.TrimSpace(result.String())

	if text == "" {
		return ""
	}

	// 文章として読めるように、最後だけ軽く整える
	if !strings.HasSuffix(text, "。") &&
		!strings.HasSuffix(text, "！") &&
		!strings.HasSuffix(text, "？") &&
		!strings.HasSuffix(text, "…") &&
		!strings.HasSuffix(text, "♪") {

		switch rand.Intn(4) {
		case 0:
			text += "。"
		case 1:
			text += "！"
		case 2:
			text += "……"
		}
	}

	return text
}
func generateMemoryPost() string {
	learnedWordsMu.RLock()

	var phraseCandidates []LearnedPhrase
	for _, phrase := range learnedPhrases {
		if isProtectedName(phrase.Text) {
			continue
		}
		if isMeaninglessLearnedText(phrase.Text) {
			continue
		}
		phraseCandidates = append(phraseCandidates, phrase)
	}

	var pairCandidates []LearnedPair
	for _, pair := range learnedPairs {
		if isProtectedName(pair.Key) || isProtectedName(pair.Value) {
			continue
		}
		if isMeaninglessLearnedText(pair.Key) ||
			isMeaninglessLearnedText(pair.Value) {
			continue
		}
		pairCandidates = append(pairCandidates, pair)
	}

	learnedWordsMu.RUnlock()

	// 覚えたフレーズを、そのまま使うこともある
	if len(phraseCandidates) > 0 && rand.Intn(100) < 30 {
		total := 0

		for _, phrase := range phraseCandidates {
			weight := phrase.Count
			if weight <= 0 {
				weight = 1
			}
			total += weight
		}

		pick := rand.Intn(total)

		for _, phrase := range phraseCandidates {
			weight := phrase.Count
			if weight <= 0 {
				weight = 1
			}

			if pick < weight {
				post := addEmoji(applyNicknames(phrase.Text))

				slog.Info("generated learned phrase",
					slog.String("post", post),
				)

				return post
			}

			pick -= weight
		}
	}
	// 覚えたペアがなければ、覚えたフレーズを使う
	if len(pairCandidates) == 0 {
		if len(phraseCandidates) > 0 {
			phrase := phraseCandidates[rand.Intn(len(phraseCandidates))]
			post := addEmoji(applyNicknames(phrase.Text))

			slog.Info("generated learned phrase",
				slog.String("post", post),
			)

			return post
		}

		return ""
	}

	// 覚えた言葉からスタート
	pair := pairCandidates[rand.Intn(len(pairCandidates))]

	parts := []string{
		pair.Key + pair.Value,
	}

	current := pair.Value

	// 内部記憶 → 外部辞書から1～2語つなぐ
	chainLength := 1 + rand.Intn(2)

	for i := 0; i < chainLength; i++ {
		next, ok := findNextWord(current)

		if !ok || next == "" {
			break
		}

		parts = append(parts, next)
		current = next
	}

	post := shapeDaseiParts(parts)

	post = applyNicknames(post)
	post = addEmoji(post)

	slog.Info("generated learned post",
		slog.String("post", post),
	)

	return post
}
func isMeaninglessLearnedText(text string) bool {
	text = strings.TrimSpace(text)

	if text == "" {
		return true
	}

	// 記号だけの文字列を除外
	meaningful := false

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			meaningful = true
			break
		}
	}

	return !meaningful
}
func findNextWord(word string) (string, bool) {
	// まず、だせい自身が覚えている言葉から探す
	learnedWordsMu.RLock()

	candidates := []string{}

	for _, pair := range learnedPairs {
		if pair.Key == word &&
			!isProtectedName(pair.Key) &&
			!isProtectedName(pair.Value) {

			candidates = append(candidates, pair.Value)
		}
	}

	learnedWordsMu.RUnlock()

	// 内部記憶にあれば、今まで通りそれを使う
	if len(candidates) > 0 {
		return candidates[rand.Intn(len(candidates))], true
	}

	// 内部記憶に無ければ外部辞書へ
	return findExternalNextWord(word)
}
func pickWord(words []string) string {
	if len(words) == 0 {
		return ""
	}

	// まず最小使用回数を探す
	min := -1
	for _, w := range words {
		c := mutterUsage[w]
		if min == -1 || c < min {
			min = c
		}
	}

	// 最小回数～最小+2回までを候補にする
	var candidates []string
	for _, w := range words {
		if mutterUsage[w] <= min+2 {
			candidates = append(candidates, w)
		}
	}

	return candidates[rand.Intn(len(candidates))]
}
func applyNicknames(text string) string {
	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	for name, nickname := range nicknames {
		text = strings.ReplaceAll(text, name+"さん", nickname)
		text = strings.ReplaceAll(text, name, nickname)
	}

	return text
}
func createMutter(text string) string {
	// 10%は何も言わない
	if rand.Intn(100) < 30 {
		return decorateMutter(createChuunibyou())
	}
	if rand.Intn(100) < 80 && len(memories) > 0 {
		memoryMu.RLock()

		values := make([]string, 0, len(memories))
		for name, m := range memories {
			nicknameMu.RLock()
			_, isOriginalName := nicknames[name]
			nicknameMu.RUnlock()

			if isOriginalName {
				continue
			}

			values = append(values, m.Value)
		}

		memoryMu.RUnlock()

		if len(values) > 0 {
			connectors := []string{"は", "が", "も", "って"}

			first := values[rand.Intn(len(values))]

			if len(values) == 1 {
				return addEmoji(applyNicknames(first))
			}
			second := values[rand.Intn(len(values))]

			reply := addEmoji(
				first +
					connectors[rand.Intn(len(connectors))] +
					second,
			)

			return decorateMutter(applyNicknames(reply))
		}
	}

	mutters := []string{
		"？",
		"はい",
		"たぶん",
		"えーとえーと",
	}

	return decorateMutter(
		applyNicknames(mutters[rand.Intn(len(mutters))]),
	)
}
func createChuunibyou() string {
	templates := []string{
		"我が右腕に宿りし【能力】……今こそ、その封印を解き放つ時だ……！",
		"我が名は【名前】……この世界を支配するために堕天した者だ。",
		"俺様の封印されし右目……今、解き放て！【記憶】の真実を見せてやる！",
		"我が右腕に宿りし紅蓮焔、左腕に宿りし蒼魂焔よ！【記憶】を灰燼へ帰せ！",
		"久遠なる虚無の狭間にて、【記憶】の慟哭を聴け……これが運命というものだ。",
		"バニッシュメント・ディス・ワールド……！【記憶】よ、我が力の前に跪け！",
		"2000年前……俺は【記憶】をこの身に封印した。だが今、その力が疼いている……！",
		"我は魔王【名前】！この世を支配するために来た！我が覇道を阻む者は容赦しない！",
		"闇夜に葬られし漆黒の【記憶】……今宵、その名を世界に刻もう……。",
		"フッ......この程度か。俺は......さ......最強だ‼ 【記憶】の力も、まだほんの一端に過ぎない......！",
	}

	memoryMu.RLock()

	values := make([]string, 0, len(memories))
	for _, m := range memories {
		values = append(values, m.Value)
	}

	memoryMu.RUnlock()

	result := templates[rand.Intn(len(templates))]

	if len(values) > 0 {
		memory := values[rand.Intn(len(values))]

		result = strings.ReplaceAll(result, "【記憶】", memory)
		result = strings.ReplaceAll(result, "【能力】", memory)
		result = strings.ReplaceAll(result, "【名前】", memory)
	} else {
		result = strings.ReplaceAll(result, "【記憶】", "")
		result = strings.ReplaceAll(result, "【能力】", "")
		result = strings.ReplaceAll(result, "【名前】", "")
	}

	result = applyNicknames(result)

	if len([]rune(result)) > 149 {
		result = string([]rune(result)[:149])
	}

	result = polishDaseiReply(result, result)

	if len([]rune(result)) > 149 {
		result = string([]rune(result)[:149])
	}

	return result
}
func isNGMember(name string) bool {
	for _, ng := range ngMembers {
		if name == ng {
			return true
		}
	}
	return false
}
func isNGWord(text string) bool {
	for _, ng := range ngWords {
		if strings.Contains(text, ng) {
			return true
		}
	}
	return false
}
func isNGAccount(account string) bool {
	for _, ng := range ngAccounts {
		if account == ng {
			return true
		}
	}
	return false
}
func rememberMember(id, name string) {
	name = strings.TrimSpace(name)
	if isNGMember(name) {
		return
	}

	if name == "" {
		return
	}

	if id == "" || name == "" {
		return
	}

	membersMu.Lock()
	defer membersMu.Unlock()

	members[id] = name
}
func (h *Handler) getMemberPosts(
	authCtx context.Context,
	communityID string,
	memberID string,
) ([]string, error) {

	if communityID == "" || memberID == "" {
		return nil, nil
	}

	resp, err := h.apiClient.GetCommunityTimeline(
		authCtx,
		&application_apiv1.GetCommunityTimelineRequest{
			CommunityId: communityID,
		},
	)
	if err != nil {
		return nil, err
	}

	var texts []string

	for _, post := range resp.GetPosts() {
		if post == nil {
			continue
		}

		if post.GetCreatorId() != memberID {
			continue
		}

		text := strings.TrimSpace(post.GetText())
		if text == "" {
			continue
		}

		texts = append(texts, text)

		if len(texts) >= 10 {
			break
		}
	}

	return texts, nil
}
func GenerateReply(text string, isMention bool) string {
	if isMention {
		if reply := mentionReply(text); reply != "" {
			return reply
		}
	}

	if reply := fixedReply(text); reply != "" {
		return reply
	}

	return createReply(text)
}

// handleChatMessage handles chat message received events by echoing the message back.
func (h *Handler) handleChatMessage(ctx context.Context, ev *modelv1.ChatMessageReceivedEvent) error {
	msg := ev.GetMessage()
	if msg == nil {
		return nil
	}

	userText := msg.GetText()
	if userText == "" {
		return nil
	}

	isMention := strings.Contains(userText, "@dasei")

	reply := GenerateReply(userText, isMention)

	authCtx, err := h.authenticator.AuthorizedContext(ctx)
	if err != nil {
		return err
	}

	_, err = h.apiClient.SendChatMessage(authCtx, &application_apiv1.SendChatMessageRequest{
		RoomId: msg.GetRoomId(),
		Text:   &reply,
	})
	if err != nil {
		return err
	}

	h.logger.Info("sent chat message",
		slog.String("room_id", msg.GetRoomId()),
		slog.String("reply", reply),
	)
	return nil
}
func randomReply(list ...string) string {
	return list[rand.Intn(len(list))]
}
func addEmoji(text string) string {
	loadEmojiDictionary()

	// 絵文字一覧の取得に失敗した場合は何もしない
	if len(emojiList) == 0 {
		return text
	}

	// 約80%の確率で絵文字を付ける
	if rand.Intn(10) < 8 {
		emoji := emojiList[rand.Intn(len(emojiList))]
		return text + emoji
	}

	return text
}
func randomLearnedWord() string {
	learnedWordsMu.RLock()
	defer learnedWordsMu.RUnlock()

	if len(learnedPairs) == 0 {
		return ""
	}

	pair := learnedPairs[rand.Intn(len(learnedPairs))]

	if rand.Intn(2) == 0 {
		return pair.Key
	}

	return pair.Value
}
