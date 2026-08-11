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
	slog.Info("EVENT TYPE", slog.Int("event_type", int(ev.EventType)))
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

			if err := SaveLearnedPair(words[i], words[i+1]); err != nil {
				slog.Error("SaveLearnedPair failed", slog.String("error", err.Error()))
			}
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

				if err := SaveLearnedPair(words[i], words[i+2]); err != nil {
					slog.Error("SaveLearnedPair failed", slog.String("error", err.Error()))
				}
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
	// 覚えたフレーズがあれば、それをそのまま使う
	if len(learnedPhrases) > 0 && rand.Intn(100) < 60 {
		var candidates []LearnedPhrase

		for _, phrase := range learnedPhrases {
			if isProtectedName(phrase.Text) {
				continue
			}

			candidates = append(candidates, phrase)
		}

		if len(candidates) > 0 {
			total := 0

			for _, phrase := range candidates {
				if phrase.Count > 0 {
					total += phrase.Count
				} else {
					total++
				}
			}

			pick := rand.Intn(total)

			for _, phrase := range candidates {
				weight := phrase.Count
				if weight <= 0 {
					weight = 1
				}

				if pick < weight {
					return phrase.Text
				}

				pick -= weight
			}
		}
	}
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
			next, ok = findExternalNextWord(current)
		}
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
		"フッ……この程度か。俺は……さ……最強だ‼️【記憶】の力も、まだほんの一端に過ぎない……！",
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
