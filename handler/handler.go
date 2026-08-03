package handler

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
)

type Memory struct {
	Value       string
	LearnedAt   time.Time
	LastReplyAt time.Time

	MutterCount int
}

var (
	members   []string
	membersMu sync.RWMutex
)
var wakati *tokenizer.Tokenizer

func init() {
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
var (
	memories = make(map[string]Memory)
	memoryMu sync.RWMutex

	mutterUsage = make(map[string]int)

	teaches = make(map[string]string)
	teachMu sync.RWMutex
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
	if rand.Intn(100) < 20 {
		return nil
	}
	reply := createMutter("")

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

	_, err = h.apiClient.CreatePost(
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

		h.logger.Info("community post",
			slog.Any("post", post),
		)
		h.logger.Info("event", slog.Any("event", ev))
		h.logger.Info("post data", slog.Any("post", post.GetPost()))
		text := post.GetPost().GetText()

		if isNGWord(text) {
			return nil
		}

		displayName := ""
		if post.GetIssuer() != nil {
			displayName = post.GetIssuer().GetDisplayName()
			rememberMember(displayName)
		}
		if displayName == "だせい" {
			return nil
		}
		if isNGMember(displayName) {
			return nil
		}
		rememberKnowledge(text)

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
		shouldReply := isMention

		if !shouldReply && rand.Intn(100) < 80 {
			shouldReply = true
		}

		if !shouldReply {
			return nil
		}
		reply := GenerateReply(
			text,
			isMention,
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
func rememberKnowledge(text string) {
	slog.Info("rememberKnowledge called")
	tokens := wakati.Tokenize(text)

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
	var words []string

	for _, token := range tokens {
		surface := strings.TrimSpace(token.Surface)

		if surface == "" {
			continue
		}

		features := token.Features()

		if len(features) == 0 {
			continue
		}

		pos := features[0]

		// 名詞だけ覚える
		if pos == "名詞" {
			words = append(words, surface)
		}
	}
	for i := 0; i < len(words)-1; i++ {

		key := words[i]
		value := words[i+1]

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
	}
}
func createReply(text string) string {
	slog.Info("createReply", slog.String("text", text))
	// 10%の確率で返信しない
	if rand.Intn(100) >= 90 {
		slog.Info("skip reply")
		return ""
	}
	if strings.HasPrefix(text, "だせい、") && strings.Contains(text, "は") && strings.Contains(text, "だよ") {
		body := strings.TrimPrefix(text, "だせい、")

		parts := strings.SplitN(body, "は", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSuffix(parts[1], "だよ")
			value = strings.TrimSpace(value)

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
	}

	memoryMu.RLock()
	slog.Info("memory count", slog.Int("count", len(memories)))
	var memory Memory
	var matchedKey string
	ok := false

	for key, m := range memories {
		slog.Info("checking",
			slog.String("key", key),
			slog.String("text", text),
		)
		slog.Info("memory count", slog.Int("count", len(memories)))
		if strings.Contains(text, key) {

			// 最近返信した記憶は使わない
			if !m.LastReplyAt.IsZero() &&
				time.Since(m.LastReplyAt) < 5*time.Minute {

				slog.Info("skip recent reply",
					slog.String("key", key),
				)
				continue
			}

			slog.Info("matched")
			memory = m
			matchedKey = key
			ok = true
			break
		}
	}

	memoryMu.RUnlock()
	if ok {
		if time.Since(memory.LearnedAt) > 10*24*time.Hour {
			memoryMu.Lock()
			delete(memories, matchedKey)
			memoryMu.Unlock()
			ok = false
		}
	}

	if ok {

		// 10%はテンプレ返信
		if rand.Intn(100) < 10 {

			switch rand.Intn(5) {
			case 0:
				return addEmoji("おお")
			case 1:
				return addEmoji("へぇ～")
			case 2:
				return addEmoji("なるほど")
			case 3:
				return addEmoji("そうなんだ")
			default:
				return addEmoji("だせいもそう思う")
			}
		}

		// 90%は記憶を返す
		memoryMu.Lock()
		memory.LastReplyAt = time.Now()
		memories[matchedKey] = memory
		memoryMu.Unlock()

		slog.Info("returning", slog.String("value", memory.Value))
		return addEmoji(memory.Value)
	}
	switch {

	case strings.Contains(text, "こんにちは"):
		return addEmoji(randomReply(
			"はろー",
			"おお",
			"んー？",
			"はい",
		))

	case strings.Contains(text, "おはよう"):
		return addEmoji(randomReply(
			"おはよう",
			"おお",
			"はろー",
		))

	case strings.Contains(text, "こんばんは"):
		return addEmoji(randomReply(
			"こんばんは",
			"わかる",
			"おお",
		))

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
func generateMemoryPost() string {
	memoryMu.RLock()
	defer memoryMu.RUnlock()
	mode := rand.Intn(100)
	if len(memories) == 0 {
		return ""
	}

	// 40%は覚えたペアをそのまま使う
	if mode < 40 {
		// 30%はペアを少し混ぜる
		if mode < 70 {

			type pair struct {
				Key   string
				Value string
			}

			var pairs []pair

			for key, mem := range memories {
				if mem.Value != "" {
					pairs = append(pairs, pair{
						Key:   key,
						Value: mem.Value,
					})
				}
			}

			if len(pairs) >= 2 {

				p1 := pairs[rand.Intn(len(pairs))]
				p2 := pairs[rand.Intn(len(pairs))]

				// 同じペアならもう一度選ぶ
				for len(pairs) > 1 && p1.Key == p2.Key {
					p2 = pairs[rand.Intn(len(pairs))]
				}

				post := p1.Key + " " + p2.Value

				mutterUsage[p1.Key]++
				mutterUsage[p2.Value]++

				slog.Info("generated mixed post",
					slog.String("post", post),
				)

				return post
			}
		}
		type pair struct {
			Key   string
			Value string
		}

		var pairs []pair

		for key, mem := range memories {
			if mem.Value != "" {
				pairs = append(pairs, pair{
					Key:   key,
					Value: mem.Value,
				})
			}
		}

		if len(pairs) > 0 {
			p := pairs[rand.Intn(len(pairs))]

			post := p.Key + " " + p.Value

			slog.Info("generated pair post",
				slog.String("post", post),
			)

			return post
		}
	}

	// ここから下は今までどおり
	var words []string

	for key, mem := range memories {
		words = append(words, key)

		if mem.Value != "" {
			words = append(words, mem.Value)
		}
	}

	rand.Shuffle(len(words), func(i, j int) {
		words[i], words[j] = words[j], words[i]
	})

	count := 2 + rand.Intn(2) // 2～3語

	if count > len(words) {
		count = len(words)
	}
	post := strings.Join(words[:count], " ")
	for _, word := range words[:count] {
		mutterUsage[word]++
	}
	slog.Info("generated memory post",
		slog.String("post", post),
	)

	return post
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

			return addEmoji(
				first +
					connectors[rand.Intn(len(connectors))] +
					second,
			)
		}
	}

	mutters := []string{
		"？",
		"はい",
		"たぶん",
		"えーとえーと",
	}

	return mutters[rand.Intn(len(mutters))]
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
func rememberMember(name string) {
	name = strings.TrimSpace(name)
	if isNGMember(name) {
		return
	}

	// 呼び捨てにする
	name = strings.TrimSuffix(name, "さん")
	name = strings.TrimSuffix(name, "ちゃん")
	name = strings.TrimSuffix(name, "君")
	name = strings.TrimSuffix(name, "くん")

	if name == "" {
		return
	}

	membersMu.Lock()
	defer membersMu.Unlock()

	for _, m := range members {
		if m == name {
			return
		}
	}

	members = append(members, name)
	slog.Info("member remembered", slog.Any("members", members))
}
func GenerateReply(text string, isMention bool) string {
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
	if !isMention && rand.Intn(100) >= 50 {
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
