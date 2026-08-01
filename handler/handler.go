package handler

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
)

type Memory struct {
	Value    string
	LastUsed time.Time
}

var (
	members   []string
	membersMu sync.RWMutex
)
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
	return &Handler{
		logger:        slog.Default(),
		apiClient:     apiClient,
		authenticator: authenticator,
	}
}
func (h *Handler) PostMutter(ctx context.Context) error {
	if rand.Intn(100) < 95 {
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
		rememberWords(text)
		if isNGWord(text) {
			return nil
		}

		displayName := ""
		if post.GetIssuer() != nil {
			displayName = post.GetIssuer().GetDisplayName()
			rememberMember(displayName)
		}

		if isNGMember(displayName) {
			return nil
		}

		account := ""
		if post.GetIssuer() != nil {
			account = post.GetIssuer().GetUserId()
		}
		if isNGAccount(account) {
			return nil
		}
		isMention := strings.Contains(text, "@dasei")

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

		if isMention {
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
func rememberWords(text string) {
	words := strings.Fields(text)

	memoryMu.Lock()
	defer memoryMu.Unlock()

	for _, word := range words {
		word = strings.TrimSpace(word)

		if len([]rune(word)) < 2 {
			continue
		}

		memories[word] = Memory{
			Value:    word,
			LastUsed: time.Now(),
		}
	}
}
func createReply(text string) string {
	slog.Info("createReply", slog.String("text", text))
	if strings.HasPrefix(text, "だせい、") && strings.Contains(text, "は") && strings.Contains(text, "だよ") {
		body := strings.TrimPrefix(text, "だせい、")

		parts := strings.SplitN(body, "は", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSuffix(parts[1], "だよ")
			value = strings.TrimSpace(value)

			memoryMu.Lock()
			memories[key] = Memory{
				Value:    value,
				LastUsed: time.Now(),
			}
			teachMu.Lock()
			teaches[key] = value
			teachMu.Unlock()
			memoryMu.Unlock()
			return addEmoji("わかった！")
		}
		teachMu.RLock()
		for key, value := range teaches {
			if strings.Contains(text, key) {
				teachMu.RUnlock()
				return addEmoji(value)
			}
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
			slog.Info("matched")
			slog.Info("reply", slog.String("value", memory.Value))
			memory = m
			slog.Info("copied", slog.Any("memory", memory))
			matchedKey = key
			ok = true
			break
		}
	}

	memoryMu.RUnlock()
	if ok {
		if time.Since(memory.LastUsed) > 10*24*time.Hour {
			memoryMu.Lock()
			delete(memories, matchedKey)
			memoryMu.Unlock()
			ok = false
		}
	}

	if ok {
		memoryMu.Lock()
		memory.LastUsed = time.Now()
		memories[matchedKey] = memory
		memoryMu.Unlock()
		slog.Info("returning", slog.String("value", memory.Value))
		return addEmoji(randomReply(
			memory.Value,
			memory.Value+"だった気がする",
			"たぶん"+memory.Value,
			"忘れそうだけど"+memory.Value,
		))
	}
	switch {

	case strings.Contains(text, "こんにちは"):
		return addEmoji(randomReply(
			"やぁ",
			"おお",
			"呼んだ？",
			"だせいいた",
		))

	case strings.Contains(text, "おはよう"):
		return addEmoji(randomReply(
			"おはよう",
			"おお",
			"朝だ",
		))

	case strings.Contains(text, "こんばんは"):
		return addEmoji(randomReply(
			"こんばんは",
			"もう夜",
			"おお",
		))

	case strings.Contains(text, "おやすみ"):
		return addEmoji(randomReply(
			"おやすみ",
			"またね",
			"寝る",
		))

	case strings.Contains(text, "疲れた"):
		return addEmoji(randomReply(
			"無理しなくていいよ",
			"おお",
			"だせいも",
			"今日は終わり",
		))

	case strings.Contains(text, "眠い"):
		return addEmoji(randomReply(
			"だせいも",
			"寝よう",
			"おお",
		))

	case strings.Contains(text, "カレー"):
		return addEmoji(randomReply(
			"飲み物",
			"うまい",
			"黄色",
		))

	case strings.Contains(text, "かわいい"):
		return addEmoji(randomReply(
			"へへ",
			"照れる",
			"おお",
		))

	case strings.Contains(text, "だせい"):
		return addEmoji(randomReply(
			"呼んだ？",
			"おお",
			"いた",
		))

	default:
		memoryMu.RLock()
		slog.Info("memory count", slog.Int("count", len(memories)))
		for _, m := range memories {
			if rand.Intn(5) == 0 {
				memoryMu.RUnlock()
				return addEmoji(m.Value)
			}
		}
		memoryMu.RUnlock()

		return addEmoji(randomReply(
			"おお",
			"なるほど",
			"へぇ",
		))
	}
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
			return addEmoji(randomReply(
				values[rand.Intn(len(values))],
				"たぶん"+values[rand.Intn(len(values))],
				values[rand.Intn(len(values))]+"だった気がする",
			))
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
