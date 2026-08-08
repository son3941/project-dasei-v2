package handler

import (
	"context"
	"log/slog"
	"math/rand"
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
	if len([]rune(reply)) > 140 {
		reply = string([]rune(reply)[:140])
	}
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

		if !shouldReply {
			return nil
		}
		slog.Info("before GenerateReply")
		reply := GenerateReply(
			text,
			isMention,
		)
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
			"community",
			slog.String("communityID", post.GetPost().GetCommunityId()),
			slog.String("postID", post.GetPost().GetPostId()),
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

	// だせいと呼ばれたら必ず返信
	if isMention {
		return true
	}

	text = strings.TrimSpace(text)

	if text == "" {
		return false
	}

	// 質問には返信しやすくする
	if strings.Contains(text, "？") ||
		strings.Contains(text, "?") ||
		strings.HasSuffix(text, "の？") ||
		strings.HasSuffix(text, "かな") ||
		strings.HasSuffix(text, "どう？") {
		return rand.Intn(100) < 70
	}

	// 会話への反応が必要そうな投稿
	replyWords := []string{
		"おはよう",
		"こんにちは",
		"こんばんは",
		"おやすみ",
		"疲れた",
		"眠い",
		"嬉しい",
		"悲しい",
		"楽しい",
		"面白い",
		"かわいい",
		"ありがとう",
		"助かった",
	}

	for _, word := range replyWords {
		if strings.Contains(text, word) {
			return rand.Intn(100) < 50
		}
	}

	// それ以外は低確率で反応
	return rand.Intn(100) < 15
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

		// 助詞・助動詞・記号は覚えない
		switch features[0] {
		case "助詞", "助動詞", "記号":
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
		case "名詞", "形容詞", "副詞", "動詞":
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
		}
		if i+2 < len(words) {

			exists = false

			for _, pair := range learnedPairs {
				if pair.Key == words[i] && pair.Value == words[i+2] {
					exists = true
					break
				}
			}

			if !exists {
				learnedPairs = append(learnedPairs, LearnedPair{
					Key:   words[i],
					Value: words[i+2],
				})
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
		if name == displayName {
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
func createReply(text string) string {

	// 名前を呼ばれたら必ず返信
	if strings.Contains(text, "だせい") || strings.Contains(text, "惰性") {
		slog.Info("force reply")

		// 返信しない判定はスキップ
	} else {
		// 10%の確率で返信しない
		if rand.Intn(100) >= 90 {
			slog.Info("skip reply")
			return ""
		}
	}
	if strings.HasPrefix(text, "だせい、") &&
		strings.Contains(text, "さんは") &&
		strings.Contains(text, "だよ") {

		body := strings.TrimPrefix(text, "だせい、")

		parts := strings.SplitN(body, "さんは", 2)
		if len(parts) == 2 {

			name := strings.TrimSpace(parts[0])
			name = strings.TrimSuffix(name, "さん")

			nickname := strings.TrimSuffix(parts[1], "だよ")
			nickname = strings.TrimSpace(nickname)

			nicknameMu.Lock()
			nicknames[name] = nickname
			nicknameMu.Unlock()

			teachMu.Lock()
			delete(teaches, name)
			teachMu.Unlock()

			return addEmoji("わかった！")
		}
	}
	nicknameMu.RLock()
	for name, nickname := range nicknames {
		text = strings.ReplaceAll(text, name+"さん", nickname)
		text = strings.ReplaceAll(text, name, nickname)
	}
	nicknameMu.RUnlock()

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

	reply := replyFromMemory(text)
	if reply != "" {
		return reply
	}

	reply = replyFromTemplate(text)
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

	switch {

	case strings.Contains(text, "おやすみ"):
		return addEmoji(randomReply(
			"おやすみ",
			"なの！",
			"ねるの？",
		))

	case strings.Contains(text, "疲れた"):
		return addEmoji(randomReply(
			"ほんと？",
			"おお",
			"だせいも",
			"わかる",
		))

	case strings.Contains(text, "眠い"):
		return addEmoji(randomReply(
			"だせいも",
			"ほんと？",
			"おお",
		))

	case strings.Contains(text, "かわいい"):
		return addEmoji(randomReply(
			"へへ",
			"えーとえーと",
			"おお",
		))

	case strings.Contains(text, "だせい"):
		if rand.Intn(100) < 70 {
			post := generateMemoryPost()
			if post != "" {
				return addEmoji(post)
			}
		}

		return addEmoji(randomReply(
			"えーとえーと",
			"おお",
			"んー？",
		))

	default:
		post := generateMemoryPost()
		if post != "" {
			return addEmoji(post)
		}

		return addEmoji(randomReply(
			"おお",
			"あ！！！",
			"えーとえーと",
		))
	}
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
func generateMemoryPost() string {

	learnedWordsMu.RLock()
	defer learnedWordsMu.RUnlock()

	if len(learnedPairs) == 0 {
		return ""
	}

	var candidates []LearnedPair

	for _, pair := range learnedPairs {
		if isProtectedName(pair.Key) || isProtectedName(pair.Value) {
			continue
		}

		candidates = append(candidates, pair)
	}

	if len(candidates) == 0 {
		return ""
	}

	pair := candidates[rand.Intn(len(candidates))]

	post := pair.Key + pair.Value

	current := pair.Value

	for i := 0; i < 3; i++ {
		next, ok := findNextWord(current)
		if !ok {
			break
		}

		post += next
		current = next
	}

	post = addEmoji(post)

	slog.Info("generated learned post",
		slog.String("post", post),
	)

	return post

}
func findNextWord(word string) (string, bool) {
	learnedWordsMu.RLock()
	defer learnedWordsMu.RUnlock()

	candidates := []string{}

	for _, pair := range learnedPairs {
		if pair.Key == word &&
			!isProtectedName(pair.Key) &&
			!isProtectedName(pair.Value) {
			candidates = append(candidates, pair.Value)
		}
	}

	if len(candidates) == 0 {
		return "", false
	}

	return candidates[rand.Intn(len(candidates))], true
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
func createMutter(text string) string {
	// 10%は何も言わない
	if rand.Intn(100) < 80 && len(memories) > 0 {
		memoryMu.RLock()

		values := make([]string, 0, len(memories))
		for _, m := range memories {
			values = append(values, m.Value)
		}

		memoryMu.RUnlock()

		if len(values) > 0 {
			connectors := []string{"は", "が", "も", "って"}

			first := values[rand.Intn(len(values))]

			if len(values) == 1 {
				return addEmoji(first)
			}

			second := values[rand.Intn(len(values))]

			reply := addEmoji(
				first +
					connectors[rand.Intn(len(connectors))] +
					second,
			)

			return decorateMutter(reply)
		}
	}

	mutters := []string{
		"？",
		"はい",
		"たぶん",
		"えーとえーと",
	}

	return decorateMutter(
		mutters[rand.Intn(len(mutters))],
	)
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
func GenerateReply(text string, isMention bool) string {
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

	if !isMention && fixedReply(userText) == "" && rand.Intn(100) >= 50 {
		return nil
	}

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
	emojis := []string{
		"🍆", "🛸", "🧦", "🪥", "🥒", "🦐", "🐟", "🪼", "🧃",
	}

	// 約80%の確率で絵文字を付ける
	if rand.Intn(10) < 8 {
		return text + emojis[rand.Intn(len(emojis))]
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
