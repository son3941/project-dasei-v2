package handler

import (
	"context"
	"encoding/json"
	"fmt"
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
var kaibunshoMutterCountMu sync.Mutex

var kaibunshoMutterCountByCommunity = map[string]int{}

func shouldPostTiredDasei(communityID string) bool {
	communityID = strings.TrimSpace(communityID)
	if communityID == "" {
		return false
	}

	kaibunshoMutterCountMu.Lock()
	defer kaibunshoMutterCountMu.Unlock()

	kaibunshoMutterCountByCommunity[communityID]++

	if kaibunshoMutterCountByCommunity[communityID] >= 10 {
		kaibunshoMutterCountByCommunity[communityID] = 0
		return true
	}

	return false
}

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
	// communityID → key → Memory
	memories = make(map[string]map[string]Memory)
	memoryMu sync.RWMutex

	// communityID → memberName → nickname
	nicknames  = make(map[string]map[string]string)
	nicknameMu sync.RWMutex

	mutterUsage = make(map[string]int)

	teaches = make(map[string]string)
	teachMu sync.RWMutex
)

var (
	// communityID → learned pairs
	learnedPairs = make(map[string][]LearnedPair)

	// 現在は実質未使用。旧仕様として残す。
	learnedPhrases []LearnedPhrase

	learnedWordsMu sync.RWMutex
)
var (
	lastHumanPostAt   = make(map[string]time.Time)
	lastHumanPostAtMu sync.RWMutex

	forcedActiveUntil   = make(map[string]time.Time)
	forcedActiveUntilMu sync.RWMutex
)

func isDaseiActive(communityID string) bool {
	lastHumanPostAtMu.RLock()
	lastHuman := lastHumanPostAt[communityID]
	lastHumanPostAtMu.RUnlock()

	forcedActiveUntilMu.RLock()
	forcedUntil := forcedActiveUntil[communityID]
	forcedActiveUntilMu.RUnlock()

	if !forcedUntil.IsZero() && time.Now().Before(forcedUntil) {
		return true
	}

	if lastHuman.IsZero() {
		return false
	}

	return time.Since(lastHuman) < time.Hour
}
func StartForcedDaseiActivity(communityID string, duration time.Duration) {
	forcedActiveUntilMu.Lock()
	forcedActiveUntil[communityID] = time.Now().Add(duration)
	forcedActiveUntilMu.Unlock()
}

// Handler implements event.EventHandler interface.
type Handler struct {
	logger        *slog.Logger
	apiClient     application_apiv1.ApplicationServiceClient
	authenticator auth.Authenticator

	communityMu  sync.RWMutex
	communityIDs []string

	daseiCreatorID string
}

// NewHandler creates a new Handler.
func NewHandler(
	apiClient application_apiv1.ApplicationServiceClient,
	authenticator auth.Authenticator,
	communityID ...string,
) *Handler {

	h := &Handler{
		logger:        slog.Default(),
		apiClient:     apiClient,
		authenticator: authenticator,
	}

	for _, id := range communityID {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}

		h.communityIDs = append(h.communityIDs, id)
	}

	for _, id := range h.communityIDs {
		h.loadCommunityMemory(id)
	}

	return h
}

func (h *Handler) loadCommunityMemory(communityID string) {
	loaded, err := LoadMemories(communityID)
	if err != nil {
		slog.Error(
			"LoadMemories failed",
			slog.String("communityID", communityID),
			slog.String("error", err.Error()),
		)
	} else {
		memoryMu.Lock()

		if memories[communityID] == nil {
			memories[communityID] = make(map[string]Memory)
		}

		for k, v := range loaded {
			memories[communityID][k] = Memory{
				Value:     v,
				LearnedAt: time.Now(),
			}
		}

		memoryMu.Unlock()
	}

	pairs, err := LoadLearnedPairs(communityID)
	if err != nil {
		slog.Error(
			"LoadLearnedPairs failed",
			slog.String("communityID", communityID),
			slog.String("error", err.Error()),
		)
	} else {
		learnedWordsMu.Lock()
		learnedPairs[communityID] = pairs
		learnedWordsMu.Unlock()
	}

	slog.Info(
		"community memory loaded",
		slog.String("communityID", communityID),
		slog.Int("memories", len(loaded)),
		slog.Int("learnedPairs", len(pairs)),
	)
}
func (h *Handler) FetchInstalledCommunityIDs(ctx context.Context) ([]string, error) {
	authCtx, err := h.authenticator.AuthorizedContext(ctx)
	if err != nil {
		return nil, err
	}

	var communityIDs []string
	var cursor string

	for {
		req := &application_apiv1.GetCommunitiesUsingApplicationRequest{}

		if cursor != "" {
			req.Cursor = &cursor
		}

		resp, err := h.apiClient.GetCommunitiesUsingApplication(
			authCtx,
			req,
		)
		if err != nil {
			return nil, err
		}

		for _, item := range resp.GetCommunitiesUsingApplication() {
			community := item.GetCommunity()
			if community == nil {
				continue
			}

			id := strings.TrimSpace(community.GetCommunityId())
			if id == "" {
				continue
			}

			communityIDs = append(communityIDs, id)
		}

		nextCursor := resp.GetNextCursor()
		if nextCursor == "" {
			break
		}

		cursor = nextCursor
	}

	return communityIDs, nil
}
func (h *Handler) CommunityIDs() []string {
	h.communityMu.RLock()
	defer h.communityMu.RUnlock()

	ids := make([]string, len(h.communityIDs))
	copy(ids, h.communityIDs)

	return ids
}
func (h *Handler) SyncInstalledCommunities(ctx context.Context) error {
	ids, err := h.FetchInstalledCommunityIDs(ctx)
	if err != nil {
		return err
	}

	existing := make(map[string]bool)

	h.communityMu.RLock()
	for _, id := range h.communityIDs {
		existing[id] = true
	}
	h.communityMu.RUnlock()

	h.communityMu.Lock()
	h.communityIDs = make([]string, len(ids))
	copy(h.communityIDs, ids)
	h.communityMu.Unlock()

	for _, id := range ids {
		if existing[id] {
			continue
		}

		h.loadCommunityMemory(id)

		slog.Info(
			"installed community added",
			slog.String("communityID", id),
		)
	}

	return nil
}
func polishDaseiMutter(reply string, style string) string {
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	// 特殊スタイルは、それぞれの表現を壊さない。
	// 通常独り言だけ本格的な文章校正を行う。
	if style != "normal" {
		return normalizeDaseiReply(reply)
	}

	wikipediaResult := searchWikipedia(reply)
	networkResult := searchWeb(reply)

	webResult := wikipediaResult + "\n" + networkResult

	slog.Info("Mutter polish search",
		slog.String("reply", reply),
		slog.String("wikipedia", wikipediaResult),
		slog.String("network", networkResult),
	)

	referenceWords := extractReferenceWords(webResult)
	referenceSentences := extractReferenceSentences(webResult)

	polished := polishWithReference(
		reply,
		reply,
		referenceWords,
		referenceSentences,
	)

	if polished != "" {
		reply = polished
	}

	reply = checkReferenceSentences(
		reply,
		referenceSentences,
	)

	reply = polishDaseiJapanese(reply)

	return normalizeDaseiReply(reply)
}
func (h *Handler) PostMutter(ctx context.Context, communityID string) error {
	if !isDaseiActive(communityID) {
		return nil
	}

	if rand.Intn(100) >= PostChance {
		return nil
	}

	var reply string
	var style string

	if shouldPostTiredDasei(communityID) {
		material := getRandomLearnedMaterial(
			communityID,
		)

		if material != "" {
			reply = material + "…"

			slog.Info("tired dasei",
				slog.String("communityId", communityID),
				slog.String("text", reply),
			)
		}
	}

	if reply == "" {
		reply = createMutter(
			communityID,
			"",
		)
		reply, style = randomStyle(reply)
		reply = polishDaseiMutter(reply, style)
		reply = normalizeDaseiPostLength(reply)

		communityWords := getLearnedMaterials(communityID)

		kaibunsho := makeKaibunsho(reply, communityWords)
		reply = limitKaibunshoLength(kaibunsho.Text)

		slog.Info("kaibunsho",
			slog.String("communityId", communityID),
			slog.String("mode", string(kaibunsho.Mode)),
			slog.Int("level", kaibunsho.Level),
			slog.Int("mixRate", kaibunsho.MixRate),
			slog.Int("contamRate", kaibunsho.ContamRate),
			slog.String("text", reply),
		)

		slog.Info("mutter style",
			slog.String("communityId", communityID),
			slog.String("style", style),
		)
	}

	if reply == "" {
		return nil
	}

	slog.Info("mutter",
		slog.String("communityId", communityID),
		slog.String("text", reply),
	)

	authCtx, err := h.authenticator.AuthorizedContext(ctx)
	if err != nil {
		return err
	}

	if communityID == "" {
		return nil
	}

	slog.Info("community", slog.String("id", communityID))

	resp, err := h.apiClient.CreatePost(
		authCtx,
		&application_apiv1.CreatePostRequest{
			CommunityId: &communityID,
			Text:        reply,
		},
	)

	if err != nil {
		h.logger.Error("failed to create mutter",
			slog.String("communityId", communityID),
			slog.String("error", err.Error()),
		)
		return err
	}

	slog.Info("create response",
		slog.Any("resp", resp),
	)

	slog.Info("created post id",
		slog.String("communityId", communityID),
		slog.String("postId", resp.Post.PostId),
	)

	slog.Info("★★★★ PostMutter END ★★★★",
		slog.String("communityId", communityID),
	)

	return nil
}
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
		communityID := post.GetPost().GetCommunityId()
		slog.Info("received text",
			slog.String("text", text),
		)

		if isNGWord(text) {
			return nil
		}

		// リセットコマンド
		if strings.TrimSpace(text) == "だせい リセット実行" {

			// このコミュの通常記憶だけ消す。
			memoryMu.Lock()
			delete(memories, communityID)
			memoryMu.Unlock()

			// このコミュで自動学習した語句だけ消す。
			learnedWordsMu.Lock()
			delete(learnedPairs, communityID)
			learnedWordsMu.Unlock()

			// このコミュのニックネームだけ消す。
			nicknameMu.Lock()
			delete(nicknames, communityID)
			nicknameMu.Unlock()

			// DB側も、このコミュのデータだけ消す。
			if err := ClearMemories(communityID); err != nil {
				slog.Error(
					"ClearMemories failed",
					slog.String("communityID", communityID),
					slog.String("error", err.Error()),
				)
			} else {
				slog.Info(
					"community memories cleared",
					slog.String("communityID", communityID),
				)
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

		lastHumanPostAtMu.Lock()
		lastHumanPostAt[communityID] = time.Now()
		lastHumanPostAtMu.Unlock()
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

			rememberKnowledge(
				communityID,
				text,
				displayName,
			)
		}

		if err := SavePost(
			communityID,
			text,
		); err != nil {
			slog.Error(
				"SavePost failed",
				slog.String("communityID", communityID),
				slog.String("error", err.Error()),
			)
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
		authCtx, err := h.authenticator.AuthorizedContext(ctx)
		if err != nil {
			return err
		}

		communityID = post.GetPost().GetCommunityId()

		memberPosts, err := h.getMemberPosts(
			authCtx,
			communityID,
			userID,
		)
		if err != nil {
			slog.Error("getMemberPosts failed",
				slog.String("error", err.Error()),
			)
			memberPosts = nil
		}

		slog.Info("member posts loaded",
			slog.String("user_id", userID),
			slog.Int("count", len(memberPosts)),
		)
		for i, memberPost := range memberPosts {
			slog.Info("member post",
				slog.Int("index", i),
				slog.String("text", memberPost),
			)
		}

		authCtx, err = h.authenticator.AuthorizedContext(ctx)
		if err != nil {
			return err
		}
		h.logger.Info(
			"post object",
			slog.Any("post", post.GetPost()),
		)
		slog.Info("post type",
			slog.String("type", fmt.Sprintf("%T", post.GetPost())),
		)
		communityID = post.GetPost().GetCommunityId()

		replyTo := post.GetPost().GetPostId()
		threadPosts, err := h.getThreadPosts(authCtx, post.GetPost())
		if err != nil {
			slog.Error("getThreadPosts failed",
				slog.String("error", err.Error()),
			)
		} else {
			slog.Info("thread posts loaded",
				slog.Int("count", len(threadPosts)),
			)

			for i, threadPost := range threadPosts {
				slog.Info("thread post",
					slog.Int("index", i),
					slog.String("post_id", threadPost.GetPostId()),
					slog.String("reply_to", threadPost.GetInReplyToPostId()),
					slog.String("text", threadPost.GetText()),
				)
			}
		}
		slog.Info("before GenerateReply")

		nickname := getMemberNickname(
			communityID,
			userID,
		)
		slog.Info("nickname before reply",
			slog.String("userID", userID),
			slog.String("displayName", displayName),
			slog.String("nickname", nickname),
		)
		reply, usedGroq := GenerateReplyWithGroq(
			authCtx,
			communityID,
			text,
			isMention,
			threadPosts,
			h.daseiCreatorID,
			nickname,
		)

		if !usedGroq &&
			nickname != "" &&
			rand.Intn(100) < 50 &&
			!strings.Contains(reply, nickname) {

			reply = nickname + "、" + reply
		}

		if !usedGroq {
			reply = ensureDaseiReplyLength(
				reply,
				text,
			)
		}

		if reply == "" {
			return nil
		}
		slog.Info("after GenerateReply",
			slog.String("reply", reply),
		)
		if reply == "" {
			return nil
		}
		if shouldReply {
			resp, err := h.apiClient.CreatePost(
				authCtx,
				&application_apiv1.CreatePostRequest{
					CommunityId: &communityID,

					Text:            reply,
					InReplyToPostId: &replyTo,
				},
			)
			if err == nil && resp != nil && resp.GetPost() != nil {
				h.daseiCreatorID = resp.GetPost().GetCreatorId()

				slog.Info("dasei reply creator",
					slog.String("creatorId", h.daseiCreatorID),
				)
			}
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
	case constv1.EventType_EVENT_TYPE_COMMUNITY_PLUGIN_MANAGED:
		managed := ev.GetCommunityPluginManagedEvent()
		if managed == nil {
			return nil
		}

		community := managed.GetCommunity()
		if community == nil {
			return nil
		}

		communityID := strings.TrimSpace(community.GetCommunityId())
		if communityID == "" {
			return nil
		}

		for _, reason := range managed.GetEventReasonList() {
			switch reason {

			case constv1.EventReason_EVENT_REASON_COMMUNITY_PLUGIN_INSTALLED:
				h.communityMu.Lock()

				alreadyExists := false
				for _, id := range h.communityIDs {
					if id == communityID {
						alreadyExists = true
						break
					}
				}

				if !alreadyExists {
					h.communityIDs = append(h.communityIDs, communityID)
				}

				h.communityMu.Unlock()

				if !alreadyExists {
					h.loadCommunityMemory(communityID)

					slog.Info(
						"community plugin installed",
						slog.String("communityID", communityID),
					)
				}

			case constv1.EventReason_EVENT_REASON_COMMUNITY_PLUGIN_UNINSTALLED:
				h.communityMu.Lock()

				filtered := make([]string, 0, len(h.communityIDs))
				for _, id := range h.communityIDs {
					if id != communityID {
						filtered = append(filtered, id)
					}
				}

				h.communityIDs = filtered
				h.communityMu.Unlock()

				slog.Info(
					"community plugin uninstalled",
					slog.String("communityID", communityID),
				)
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
func rememberKnowledge(
	communityID string,
	text string,
	displayName string,
) {

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

		pairs := learnedPairs[communityID]

		exists := false

		for _, pair := range pairs {
			if pair.Key == words[i] &&
				pair.Value == words[i+1] {
				exists = true
				break
			}
		}

		if !exists {
			learnedPairs[communityID] = append(
				learnedPairs[communityID],
				LearnedPair{
					Key:   words[i],
					Value: words[i+1],
				},
			)

			if err := SaveLearnedPair(
				communityID,
				words[i],
				words[i+1],
			); err != nil {
				slog.Error(
					"SaveLearnedPair failed",
					slog.String("communityID", communityID),
					slog.String("key", words[i]),
					slog.String("value", words[i+1]),
					slog.String("error", err.Error()),
				)
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

	for _, communityNicknames := range nicknames {
		for originalName, nickname := range communityNicknames {
			if name == originalName || name == nickname {
				nicknameMu.RUnlock()
				return true
			}
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
func extractConversationWords(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	tokens := wakati.Tokenize(text)

	var words []string

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" ||
			word == "BOS" ||
			word == "EOS" ||
			isPolishSymbol(word) {
			continue
		}

		if len([]rune(word)) < 2 {
			continue
		}

		features := token.Features()
		if len(features) == 0 {
			continue
		}

		pos := features[0]

		switch pos {
		case "名詞", "動詞", "形容詞", "副詞":
			words = append(words, word)
		}
	}

	return words
}
func hasSharedConversationWord(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	for _, wordA := range a {
		for _, wordB := range b {
			if wordA == wordB {
				return true
			}
		}
	}

	return false
}
func buildDaseiCorrectionReply(
	text string,
	previousHumanText string,
	previousDaseiText string,
) string {
	text = strings.TrimSpace(text)
	previousHumanText = strings.TrimSpace(previousHumanText)
	previousDaseiText = strings.TrimSpace(previousDaseiText)

	if text == "" || previousDaseiText == "" {
		return ""
	}

	// 訂正の先頭語だけ外して、相手が実際に言っている内容は壊さず残す。
	correctedText := text

	for _, prefix := range []string{
		"いや、",
		"いや",
		"でも、",
		"でも",
	} {
		if strings.HasPrefix(correctedText, prefix) {
			correctedText = strings.TrimSpace(
				strings.TrimPrefix(correctedText, prefix),
			)
			break
		}
	}

	correctedText = strings.Trim(correctedText, " 、。！？")

	if correctedText == "" {
		correctedText = previousHumanText
	}

	if correctedText == "" {
		return "だせい勘違いしてたわ。"
	}

	return "そっか、" + correctedText + "。さっきはだせい勘違いしてたわ。"
}
func buildDaseiContinuationReply(
	text string,
	previousHumanText string,
	previousDaseiText string,
	mainTopic string,
) string {
	text = strings.TrimSpace(text)
	previousHumanText = strings.TrimSpace(previousHumanText)
	previousDaseiText = strings.TrimSpace(previousDaseiText)
	mainTopic = strings.TrimSpace(mainTopic)

	if text == "" {
		return ""
	}

	// 現在の発言が質問なら質問処理へ渡す。
	if isQuestionText(text) {
		topic := extractContinuationTopic(
			text,
			previousHumanText,
			previousDaseiText,
		)

		if topic == "" {
			topic = mainTopic
		}

		return buildReplyToQuestion(text, topic)
	}

	// 現在の発言だけから、新しく出た話題を取る。
	currentTopic := extractContinuationTopic(
		text,
		"",
		"",
	)

	cleanText := cleanContinuationText(text)
	if cleanText == "" {
		return ""
	}

	// 直前のだせいが質問していた場合は、
	// 今の発言をその回答として扱う。
	if previousDaseiText != "" &&
		(strings.Contains(previousDaseiText, "？") ||
			strings.Contains(previousDaseiText, "?")) {

		return "なるほど、" + cleanText + "ってことか。"
	}
	// スレッド全体の主題を保持する。
	if mainTopic != "" {
		// 主題とは別の名詞が新しく出た場合も、
		// 主題を捨てず追加情報として扱う。
		if currentTopic != "" && currentTopic != mainTopic {
			return mainTopic + "の話で、" +
				cleanText + "ってことか。"
		}

		// 現在の発言に名詞がなくても、
		// スレッドの主題へつなげる。
		if currentTopic == "" {
			return mainTopic + "の話で、" +
				cleanText + "ってことか。"
		}

		// 現在も同じ主題を明示している場合。
		return cleanText + "ってことか。"
	}

	// 主題を取得できなかった場合も、
	// 発言そのものを壊して作り直さない。
	return cleanText + "ってことか。"
}
func extractContinuationTopic(
	text string,
	previousHumanText string,
	previousDaseiText string,
) string {
	// まず現在の発言から話題を探す。
	for _, source := range []string{
		text,
		previousHumanText,
		previousDaseiText,
	} {
		tokens := wakati.Tokenize(source)

		var nounParts []string

		for _, token := range tokens {
			word := strings.TrimSpace(token.Surface)

			if word == "" ||
				word == "BOS" ||
				word == "EOS" ||
				isPolishSymbol(word) {
				continue
			}

			features := token.Features()
			if len(features) == 0 {
				continue
			}

			if features[0] == "名詞" {
				nounParts = append(nounParts, word)
				continue
			}

			if len(nounParts) > 0 {
				break
			}
		}

		if len(nounParts) > 0 {
			return strings.Join(nounParts, "")
		}
	}

	return ""
}

func cleanContinuationText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Trim(text, " 、。！？!?")

	for _, suffix := range []string{
		"んだよね",
		"んやけど",
		"だよね",
		"んだよ",
		"んやで",
		"よね",
		"んよ",
		"だよ",
		"ね",
		"よ",
	} {
		if strings.HasSuffix(text, suffix) {
			text = strings.TrimSpace(
				strings.TrimSuffix(text, suffix),
			)
			break
		}
	}

	return strings.Trim(text, " 、。！？!?")
}
func buildReplyFromHistory(
	text string,
	previousHumanText string,
	previousDaseiText string,
	mainTopic string,
) string {
	text = strings.TrimSpace(text)
	previousHumanText = strings.TrimSpace(previousHumanText)
	previousDaseiText = strings.TrimSpace(previousDaseiText)

	if text == "" {
		return ""
	}

	if previousHumanText == "" && previousDaseiText == "" {
		return ""
	}

	currentWords := extractConversationWords(text)
	humanWords := extractConversationWords(previousHumanText)
	daseiWords := extractConversationWords(previousDaseiText)

	// だせいへの訂正・ツッコミを最優先する。
	if previousDaseiText != "" {
		correctingDasei :=
			strings.HasPrefix(text, "いや") ||
				strings.Contains(text, "違う") ||
				strings.Contains(text, "ちゃう")

		if correctingDasei {
			return buildDaseiCorrectionReply(
				text,
				previousHumanText,
				previousDaseiText,
			)
		}
	}

	continued := false
	// 直前の人間発言と共通する語があれば会話継続。
	for _, humanWord := range humanWords {
		for _, currentWord := range currentWords {
			if humanWord == currentWord {
				continued = true
				break
			}
		}

		if continued {
			break
		}
	}

	// 直前のだせい発言と共通する語も確認する。
	if !continued {
		for _, daseiWord := range daseiWords {
			for _, currentWord := range currentWords {
				if daseiWord == currentWord {
					continued = true
					break
				}
			}

			if continued {
				break
			}
		}
	}
	// 接続表現・応答表現なら会話の続きとして扱う。
	if strings.Contains(text, "まだ") ||
		strings.HasPrefix(text, "でも") ||
		strings.HasPrefix(text, "せや") ||
		strings.HasPrefix(text, "そう") ||
		strings.HasPrefix(text, "なら") ||
		strings.HasPrefix(text, "じゃあ") ||
		strings.HasPrefix(text, "それ") {
		continued = true
	}

	// 直前のだせいが質問していたら、
	// 次の人間発言は基本的にその回答として扱う。
	if previousDaseiText != "" &&
		(strings.Contains(previousDaseiText, "？") ||
			strings.Contains(previousDaseiText, "?")) {
		continued = true
	}

	if !continued {
		return ""
	}

	return buildDaseiContinuationReply(
		text,
		previousHumanText,
		previousDaseiText,
		mainTopic,
	)
}
func isQuestionText(text string) bool {
	text = strings.TrimSpace(text)

	if text == "" {
		return false
	}

	if strings.Contains(text, "？") ||
		strings.Contains(text, "?") {
		return true
	}

	for _, ending := range []string{
		"かな",
		"かね",
		"なの",
		"なん",
		"んか",
		"ん？",
	} {
		if strings.HasSuffix(text, ending) {
			return true
		}
	}

	return false
}
func buildReplyToQuestion(
	text string,
	topic string,
) string {
	text = strings.TrimSpace(text)
	topic = strings.TrimSpace(topic)

	if text == "" {
		return ""
	}

	tokens := wakati.Tokenize(text)

	evaluation := ""

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" {
			continue
		}

		features := token.Features()
		if len(features) == 0 {
			continue
		}

		if features[0] == "形容詞" {
			evaluation = word
			break
		}
	}
	action := ""

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" {
			continue
		}

		features := token.Features()
		if len(features) == 0 {
			continue
		}

		if features[0] == "動詞" {
			action = word

			if len(features) > 6 {
				base := strings.TrimSpace(features[6])
				if base != "" && base != "*" {
					action = base
				}
			}

			break
		}
	}
	if evaluation != "" {
		replies := []string{
			topic + "はどうやろな。だせいはまだ分からん。",
			"だせいは" + topic + "が" + evaluation + "かまでは分からんな。",
			topic + "か。実際どうなんやろな。",
		}

		return replies[rand.Intn(len(replies))]
	}

	if action != "" {
		switch action {
		case "知る", "分かる":
			if topic == "" {
				return "それはだせいもよく分からんな。"
			}
			return topic + "か。だせいも詳しくは知らんな。"
		}

		if topic == "" {
			return "だせいはまだやな。"
		}

		return topic + "か。だせいはまだやな。"
	}
	if topic == "" {
		return "どうなんやろな。"
	}

	return topic + "のことか。"
}

func buildReplyFromCurrentText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	tokens := wakati.Tokenize(text)

	topic := ""
	var nounParts []string

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" ||
			word == "BOS" ||
			word == "EOS" ||
			isPolishSymbol(word) {
			continue
		}

		features := token.Features()
		if len(features) == 0 {
			continue
		}

		if features[0] == "名詞" {
			nounParts = append(nounParts, word)
			continue
		}

		if len(nounParts) > 0 {
			break
		}
	}

	if len(nounParts) > 0 {
		topic = strings.Join(nounParts, "")
	}

	evaluation := ""
	hasAction := false

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" {
			continue
		}

		features := token.Features()
		if len(features) == 0 {
			continue
		}

		switch features[0] {
		case "形容詞":
			if evaluation == "" {
				evaluation = word
			}

		case "動詞":
			hasAction = true
		}
	}
	if isQuestionText(text) {
		return buildReplyToQuestion(text, topic)
	}

	cleanText := strings.Trim(text, " 、。！？!?")

	if cleanText == "" {
		return "そうなんや。"
	}

	// 名詞＋評価が取れた場合。
	if evaluation != "" && topic != "" {
		replies := []string{
			topic + "、" + evaluation + "んやな。",
			topic + "が" + evaluation + "ってことか。",
		}

		return replies[rand.Intn(len(replies))]
	}

	// 名詞が取れない評価文は、無理に評価語を話題扱いしない。
	if evaluation != "" {
		return cleanText + "ってことか。"
	}

	// 動詞は活用し直さず、元の発言をそのまま材料にする。
	if hasAction {
		return cleanText + "ってことか。"
	}

	if topic != "" {
		return topic + "の話なんやな。"
	}

	return "なるほどな。そういうことか。"
}
func extractMainTopicFromHumanContext(humanContext string) string {
	humanContext = strings.TrimSpace(humanContext)
	if humanContext == "" {
		return ""
	}

	for _, line := range strings.Split(humanContext, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		topic := extractContinuationTopic(line, "", "")
		if topic != "" {
			return topic
		}
	}

	return ""
}
func generateRandomDaseiDraft(text string, threadContext ...string) string {
	text = strings.TrimSpace(text)

	var humanContext string
	var daseiContext string

	if len(threadContext) > 1 {
		humanContext = strings.TrimSpace(threadContext[1])
	}
	if len(threadContext) > 2 {
		daseiContext = strings.TrimSpace(threadContext[2])
	}

	var previousHumanText string
	if humanContext != "" {
		humanParts := strings.Split(humanContext, "\n")
		skippedCurrent := false

		for i := len(humanParts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(humanParts[i])
			if part == "" {
				continue
			}

			if !skippedCurrent && part == text {
				skippedCurrent = true
				continue
			}

			previousHumanText = part
			break
		}
	}

	var previousDaseiText string
	if daseiContext != "" {
		daseiParts := strings.Split(daseiContext, "\n")
		for i := len(daseiParts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(daseiParts[i])
			if part != "" {
				previousDaseiText = part
				break
			}
		}
	}
	mainTopic := extractMainTopicFromHumanContext(humanContext)

	historyReply := buildReplyFromHistory(
		text,
		previousHumanText,
		previousDaseiText,
		mainTopic,
	)

	if historyReply != "" {
		return historyReply
	}
	if text == "" {
		return ""
	}

	// 「どうした？」系。
	// 質問への回答ではなく、直前の自分の発言について聞かれている。
	if strings.Contains(text, "どうした") ||
		strings.Contains(text, "どした") {

		replies := []string{
			"えーとえーと",
			"なんでもないよ。ちょっと気になっただけ。",
			"だせいも自分で言ってから、なんでそう言ったのか分からん。",
		}

		return replies[rand.Intn(len(replies))]
	}
	// 「今〜だった？」系。
	// 直前のだせいの発言について確認されている。
	if strings.Contains(text, "今") &&
		(strings.Contains(text, "あった") ||
			strings.Contains(text, "言った") ||
			strings.Contains(text, "した")) {

		replies := []string{
			"いや、よく考えたらそんなことなかった。",
			"だせいが勝手にそう思っただけかもしれん。",
			"あ！！！",
		}

		return replies[rand.Intn(len(replies))]
	}
	// 否定・ツッコミ系。
	// 直前のだせいの発言に対して否定された場合。
	if strings.Contains(text, "ちゃう") ||
		strings.Contains(text, "違う") ||
		strings.Contains(text, "要らん") ||
		strings.Contains(text, "いらん") ||
		strings.Contains(text, "なんでや") {

		replies := []string{
			"まじでか",
			"そっちじゃなかったか。",
			"だせい勘違いしてたわ。",
			"なるほど、そういうことか。",
			"な　ん　や　て",
		}

		return replies[rand.Intn(len(replies))]
	}
	// ごく一部だけ短文を許容する。
	if rand.Intn(100) < 15 {
		short := []string{
			"へえ、そうなんや。",
			"なるほどなあ。",
			"知らんかった。",
			"そういうこと？",
			"なんか気になるな。",
			"そうなんやね。",
			"ふむふむ。",
		}

		return short[rand.Intn(len(short))]
	}

	// 相手から訂正・ツッコミを受けた場合。
	if strings.HasPrefix(text, "いや") ||
		strings.Contains(text, "ちゃう") ||
		strings.Contains(text, "違う") {

		replies := []string{
			"たしかに、そう言われると違うな。だせい何言ってたんやろ。",
			"あ、たしかにそうやな。なんか勝手に話を変な方向へ持っていってた。",
			"そう言われるとその通りやな。だせいちょっと勘違いしてたわ。",
		}

		return replies[rand.Intn(len(replies))]
	}

	return buildReplyFromCurrentText(text)
}
func generateReplyWithWebCheck(text string) string {
	// まず、だせいの雑な返事を作る
	reply := generateNaturalReply(text)

	if reply == "" {
		return ""
	}

	// Wikipedia＋ネット情報を取得
	wikipediaResult := searchWikipedia(text)
	networkResult := searchWeb(text)

	webResult := wikipediaResult + "\n" + networkResult

	slog.Info("Reply reference search",
		slog.String("original", text),
		slog.String("reply", reply),
		slog.String("wikipedia", wikipediaResult),
		slog.String("network", networkResult),
	)

	// 検索結果から校正に使える情報を抽出
	referenceWords := extractReferenceWords(webResult)
	referenceSentences := extractReferenceSentences(webResult)

	// ここで初めて、だせいの雑な返信を校正する
	reply = polishWithReference(
		reply,
		text,
		referenceWords,
		referenceSentences,
	)

	// 最終的な文章整理
	reply = checkReferenceSentences(
		reply,
		referenceSentences,
	)

	return normalizeDaseiReply(reply)
}
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
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	// 検索結果の文章をそのまま使っている場合は、
	// 元の文章を優先して残す。
	for _, sentence := range referenceSentences {
		sentence = strings.TrimSpace(sentence)

		if sentence == "" {
			continue
		}

		// 長すぎる検索文そのものが返信に入っていたら除去する。
		if len([]rune(sentence)) >= 30 &&
			strings.Contains(reply, sentence) {

			reply = strings.ReplaceAll(reply, sentence, "")
			reply = strings.TrimSpace(reply)

			slog.Info("Removed raw reference sentence",
				slog.String("sentence", sentence),
			)
		}
	}

	return normalizeDaseiReply(reply)
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

type polishToken struct {
	Surface string
	POS     string
}

func tokenizeForJapanesePolish(text string) []polishToken {
	text = strings.TrimSpace(text)

	if text == "" {
		return nil
	}

	tokens := wakati.Tokenize(text)

	var result []polishToken

	for _, token := range tokens {
		surface := strings.TrimSpace(token.Surface)

		if surface == "" ||
			surface == "BOS" ||
			surface == "EOS" {
			continue
		}

		if isPolishSymbol(surface) {
			continue
		}

		pos := ""
		posList := token.POS()

		if len(posList) > 0 {
			pos = posList[0]
		}

		result = append(result, polishToken{
			Surface: surface,
			POS:     pos,
		})
	}

	return result
}
func needsJapanesePolish(text string) bool {
	text = strings.TrimSpace(text)

	if text == "" {
		return false
	}

	tokens := tokenizeForJapanesePolish(text)

	if len(tokens) == 0 {
		return false
	}

	nouns := 0
	verbs := 0
	adjectives := 0
	particles := 0

	for _, token := range tokens {
		switch token.POS {
		case "名詞":
			nouns++
		case "動詞":
			verbs++
		case "形容詞":
			adjectives++
		case "助詞":
			particles++
		}
	}

	// 名詞が多く、助詞がほとんど無い場合。
	// 動詞が少し混ざっていても校正対象にする。
	if nouns >= 5 && particles <= 2 {
		return true
	}

	// 長い文章なのに句点などが一切なく、
	// 名詞の割合が高い場合も単語列とみなす。
	hasSentenceEnd := strings.ContainsAny(text, "。！？!?")

	if len(tokens) >= 10 &&
		!hasSentenceEnd &&
		nouns*2 >= len(tokens) {
		return true
	}

	// かなり長いのに助詞が少なすぎる場合。
	if len(tokens) >= 12 && particles <= 2 {
		return true
	}

	// 動詞・形容詞が全くなく名詞中心なら当然校正する。
	if nouns >= 4 &&
		verbs == 0 &&
		adjectives == 0 {
		return true
	}

	return false
}
func rebuildDaseiJapanese(text string) string {
	tokens := tokenizeForJapanesePolish(text)

	if len(tokens) == 0 {
		return text
	}

	var nouns []string
	var verbs []string
	var adjectives []string

	seen := make(map[string]bool)

	for _, token := range tokens {
		word := strings.TrimSpace(token.Surface)

		if word == "" ||
			word == "BOS" ||
			word == "EOS" ||
			seen[word] {
			continue
		}

		seen[word] = true

		switch token.POS {
		case "名詞":
			nouns = append(nouns, word)

		case "動詞":
			verbs = append(verbs, word)

		case "形容詞":
			adjectives = append(adjectives, word)
		}
	}

	if len(nouns) == 0 {
		return text
	}

	rand.Shuffle(len(nouns), func(i, j int) {
		nouns[i], nouns[j] = nouns[j], nouns[i]
	})

	rand.Shuffle(len(verbs), func(i, j int) {
		verbs[i], verbs[j] = verbs[j], verbs[i]
	})

	rand.Shuffle(len(adjectives), func(i, j int) {
		adjectives[i], adjectives[j] = adjectives[j], adjectives[i]
	})
	var sentences []string

	// 名詞同士から話題を1文作る。
	if len(nouns) >= 2 {
		first := nouns[0]
		second := nouns[1]

		openers := []string{
			first + "って" + second + "と関係あるんかな。",
			first + "の話に" + second + "まで出てくるんやね。",
			first + "と" + second + "が並ぶと、なんか気になるな。",
		}

		sentences = append(
			sentences,
			openers[rand.Intn(len(openers))],
		)
	} else {
		sentences = append(
			sentences,
			nouns[0]+"ってなんか気になるな。",
		)
	}

	// 動詞が取れていれば、別の名詞と組み合わせる。
	if len(verbs) > 0 && len(nouns) >= 3 {
		noun := nouns[2]
		verb := verbs[0]

		verbSentences := []string{
			noun + "を" + verb + "っていう話もあるんやね。",
			noun + "が" + verb + "って聞くと、ちょっと意外やな。",
			noun + "まで" + verb + "となると、だせいにはよく分からん。",
		}

		sentences = append(
			sentences,
			verbSentences[rand.Intn(len(verbSentences))],
		)
	}
	// 形容詞が取れていれば、文章の材料として使う。
	if len(adjectives) > 0 {
		adjective := adjectives[0]

		if len(nouns) >= 4 {
			noun := nouns[3]

			sentences = append(
				sentences,
				noun+"が"+adjective+"っていうのも面白いな。",
			)
		}
	}

	// まだ使っていない名詞があれば、
	// 最後に軽く話題を飛ばす。
	if len(nouns) >= 5 && rand.Intn(100) < 60 {
		noun := nouns[4]

		endings := []string{
			noun + "のことも少し気になってきた。",
			"そういえば" + noun + "ってのもあるな。",
			noun + "まで出てくるとは思わんかった。",
		}

		sentences = append(
			sentences,
			endings[rand.Intn(len(endings))],
		)
	}

	if len(sentences) == 0 {
		return text
	}

	result := strings.Join(sentences, "")

	return strings.TrimSpace(result)
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
func polishWithReference(
	reply string,
	originalText string,
	referenceWords []string,
	referenceSentences []string,
) string {
	reply = strings.TrimSpace(reply)
	originalText = strings.TrimSpace(originalText)

	if reply == "" {
		return ""
	}

	// 検索結果の文章そのものは使わない。
	// referenceSentences は今後の怪文書生成などで利用する可能性があるため、
	// この段階では受け取るだけにする。
	_ = referenceSentences

	if originalText == "" || len(referenceWords) == 0 {
		return normalizeDaseiReply(reply)
	}
	originalTokens := tokenizeForPolish(originalText)

	if len(originalTokens) == 0 {
		return normalizeDaseiReply(reply)
	}

	usefulWords := extractUsefulReferenceWords(referenceWords)

	var matchedWords []string

	for _, word := range usefulWords {
		word = strings.TrimSpace(word)

		if word == "" {
			continue
		}

		for _, token := range originalTokens {
			if token == word {
				matchedWords = append(matchedWords, word)
				break
			}
		}
	}
	// 重複を除く。
	seen := make(map[string]bool)
	uniqueWords := make([]string, 0, len(matchedWords))

	for _, word := range matchedWords {
		if seen[word] {
			continue
		}

		seen[word] = true
		uniqueWords = append(uniqueWords, word)
	}

	matchedWords = uniqueWords

	if len(matchedWords) > 3 {
		matchedWords = matchedWords[:3]
	}

	// 現段階では、検索結果の文章を固定文へ変換しない。
	// 元の下書きをそのまま最終校正へ渡す。
	slog.Info("Dasei reference words collected",
		slog.Int("matched_words", len(matchedWords)),
		slog.String("before", reply),
	)

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

	// 返信にすでに含まれている検索語は除外する。
	var newWords []string

	for _, word := range usefulWords {
		word = strings.TrimSpace(word)

		if word == "" {
			continue
		}

		if len([]rune(word)) < 2 {
			continue
		}

		if strings.Contains(reply, word) {
			continue
		}

		newWords = append(newWords, word)

		if len(newWords) >= 3 {
			break
		}
	}

	if len(newWords) == 0 {
		return reply
	}
	// 検索語を単純に羅列せず、
	// 元の返信に自然な補足として接続する。
	for _, word := range newWords {
		candidates := []string{
			reply + " " + word + "の話も気になるね。",
			reply + " " + word + "ってところが面白いね。",
			reply + " " + word + "のことも少し気になるな。",
		}

		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)

			if len([]rune(candidate)) <= 140 {
				reply = candidate
				break
			}
		}

		if len([]rune(reply)) >= 50 {
			break
		}
	}

	slog.Info("Wikipedia reference words applied",
		slog.String("reply", reply),
		slog.Int("usefulWords", len(newWords)),
	)

	return normalizeDaseiReply(reply)
}
func enrichDaseiReply(reply string, originalText string, referenceWords []string, referenceSentences []string) string {
	reply = strings.TrimSpace(reply)
	originalText = strings.TrimSpace(originalText)

	if reply == "" || originalText == "" {
		return reply
	}

	// この関数では返信を生成・水増ししない。
	// 検索結果は、元投稿と返信の関連性を確認するためだけに使う。
	replyTokens := tokenizeForPolish(reply)
	originalTokens := tokenizeForPolish(originalText)

	if len(replyTokens) == 0 || len(originalTokens) == 0 {
		return reply
	}

	matched := countRelevantCommonWords(replyTokens, originalTokens)

	slog.Info("Dasei reply relevance check",
		slog.Int("matched", matched),
		slog.Int("referenceWords", len(referenceWords)),
		slog.Int("referenceSentences", len(referenceSentences)),
		slog.String("reply", reply),
	)

	return reply
}
func normalizeDaseiPostLength(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}

	return text
}
func cleanRepeatedDaseiReply(reply string) string {
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	// 同じ文章が連続している場合だけ、
	// 後ろの重複を1回だけ削除する。
	for {
		old := reply

		parts := strings.FieldsFunc(reply, func(r rune) bool {
			return r == '。' || r == '！' || r == '？'
		})

		if len(parts) < 2 {
			break
		}

		changed := false

		var result strings.Builder

		for i, part := range parts {
			part = strings.TrimSpace(part)

			if part == "" {
				continue
			}

			if i > 0 {
				previous := strings.TrimSpace(parts[i-1])

				if part == previous {
					changed = true
					continue
				}
			}

			if result.Len() > 0 {
				result.WriteString("。")
			}

			result.WriteString(part)
		}

		if !changed {
			break
		}

		reply = result.String()

		if reply == old {
			break
		}
	}

	return reply
}
func normalizeDaseiReply(reply string) string {
	slog.Info("Dasei normalize CALLED", slog.String("reply", reply))

	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	// 連続する空白を整理。
	reply = strings.Join(strings.Fields(reply), " ")

	// 連続する句読点だけを整理。
	// 「だせい」の前後に句点を強制的に追加することはしない。
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
func ensureDaseiReplyLength(reply string, originalText string) string {
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	runes := []rune(reply)

	// 140文字以内ならそのまま返す。
	if len(runes) <= 140 {
		return reply
	}

	// 140文字以内の範囲で、
	// できるだけ後ろにある自然な文末を探す。
	limit := runes[:140]

	for i := len(limit) - 1; i >= 0; i-- {
		switch limit[i] {
		case '。', '！', '？':
			// あまりにも短くなりすぎる場合は使わない。
			if i >= 50 {
				return strings.TrimSpace(string(limit[:i+1]))
			}
		}
	}

	// 文末記号が無い場合は「、」も候補にする。
	for i := len(limit) - 1; i >= 0; i-- {
		if limit[i] == '、' && i >= 50 {
			return strings.TrimSpace(string(limit[:i])) + "。"
		}
	}

	// 適切な区切りが見つからない場合だけ140文字で切る。
	return strings.TrimSpace(string(limit))
}
func createReply(
	communityID string,
	text string,
	threadContext ...string,
) string {

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

			if nicknames[communityID] == nil {
				nicknames[communityID] = make(map[string]string)
			}

			nicknames[communityID][name] = nickname

			nicknameMu.Unlock()
			if err := SaveNickname(
				communityID,
				name,
				nickname,
			); err != nil {
				slog.Error("SaveNickname failed",
					slog.String("name", name),
					slog.String("error", err.Error()),
				)
			} else {
				slog.Info("SaveNickname success",
					slog.String("name", name),
					slog.String("nickname", nickname),
				)
			}
			teachMu.Lock()
			delete(teaches, name)
			teachMu.Unlock()

			// このコミュで覚えた、この名前を含む学習ペアだけ除去する。
			learnedWordsMu.Lock()

			communityPairs := learnedPairs[communityID]

			filteredPairs := make([]LearnedPair, 0, len(communityPairs))

			for _, pair := range communityPairs {
				if pair.Key == name || pair.Value == name {
					continue
				}

				filteredPairs = append(filteredPairs, pair)
			}

			learnedPairs[communityID] = filteredPairs

			learnedWordsMu.Unlock()

			// DB側からも、このコミュの該当学習ペアだけ削除する。
			if err := DeleteLearnedPairsByName(
				communityID,
				name,
			); err != nil {
				slog.Error(
					"DeleteLearnedPairsByName failed",
					slog.String("communityID", communityID),
					slog.String("name", name),
					slog.String("error", err.Error()),
				)
			}

			return finalizeDaseiReply(text, addEmoji("わかった！"))
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

			if memories[communityID] == nil {
				memories[communityID] = make(map[string]Memory)
			}

			memories[communityID][key] = Memory{
				Value:     value,
				LearnedAt: time.Now(),
			}

			memoryMu.Unlock()

			if err := SaveMemory(communityID, key, value); err != nil {
				slog.Error("SaveMemory failed",
					slog.String("error", err.Error()),
				)
			} else {
				slog.Info("SaveMemory success",
					slog.String("key", key),
					slog.String("value", value),
				)
			}

			return finalizeDaseiReply(text, addEmoji("わかった！"))
		}
	} // ← これを追加
	text = applyNicknames(
		communityID,
		text,
	)
	reply := replyFromMemory(
		communityID,
		text,
	)
	if reply != "" {
		return finalizeDaseiReply(text, reply)
	}

	// 返信の10%だけインプレゾンビ
	// 返信の10%だけインプレゾンビ
	if rand.Intn(100) < 10 {

		mode := rand.Intn(100)

		switch {

		case mode < 30:
			return finalizeDaseiReply(text, zombieEnglish())
		case mode < 60:
			return finalizeDaseiReply(text, zombieJapanese())

		default:
			return finalizeDaseiReply(text, zombieReply(text))
		}
	}
	// 自然な日本語で返信。
	// スレッド履歴がある場合は、現在の発言だけでなく
	// 会話の先頭からの流れも返信材料にする。
	reply = generateRandomDaseiDraft(text, threadContext...)

	if reply != "" {
		return finalizeDaseiReply(text, reply)
	}

	return finalizeDaseiReply(text, "そうなんだね")
}
func finalizeDaseiReply(originalText string, reply string) string {
	reply = strings.TrimSpace(reply)

	if reply == "" {
		return ""
	}

	// まず検索結果から得た言葉を使って内容を具体化する。
	// ここでは文章そのものを引用しない。
	reply = polishDaseiReply(originalText, reply)

	// 重複した表現だけを整理する。
	reply = cleanRepeatedDaseiReply(reply)

	// reply = polishDaseiJapanese(reply)

	// 最後に文字数上限だけを適用する。
	reply = ensureDaseiReplyLength(reply, originalText)

	return normalizeDaseiReply(reply)
}
func polishDaseiJapanese(text string) string {
	text = strings.TrimSpace(text)

	if text == "" {
		return ""
	}

	// 単語の羅列など、日本語として崩れている場合だけ
	// 読める文章へ組み直す。
	if needsJapanesePolish(text) {
		rebuilt := rebuildDaseiJapanese(text)

		if strings.TrimSpace(rebuilt) != "" {
			text = rebuilt
		}
	}

	// 改行・連続空白を整理する。
	fields := strings.Fields(text)
	text = strings.Join(fields, " ")

	if text == "" {
		return ""
	}
	// 日本語では単語間の空白を基本的に残さない。
	text = strings.ReplaceAll(text, " ", "")

	// 句読点の前後を整理する。
	text = strings.ReplaceAll(text, "。", "。 ")
	text = strings.ReplaceAll(text, "！", "！ ")
	text = strings.ReplaceAll(text, "？", "？ ")

	text = strings.Join(strings.Fields(text), " ")

	text = strings.ReplaceAll(text, "。 ", "。")
	text = strings.ReplaceAll(text, "！ ", "！")
	text = strings.ReplaceAll(text, "？ ", "？")
	// 明らかな重複だけを整理する。
	duplicatePatterns := []string{
		"そうなんやそうなんや",
		"知らんかった知らんかった",
		"なるほどなるほど",
		"へえへえ",
		"たしかにたしかに",
		"なんかなんか",
		"ふむふむふむ",
	}

	for _, pattern := range duplicatePatterns {
		half := []rune(pattern)
		if len(half) < 2 {
			continue
		}

		mid := len(half) / 2
		first := string(half[:mid])
		second := string(half[mid:])

		text = strings.ReplaceAll(text, first+second, first)
	}
	// 句読点が連続しすぎる場合だけ整理する。
	for strings.Contains(text, "。。") {
		text = strings.ReplaceAll(text, "。。", "。")
	}

	for strings.Contains(text, "、、") {
		text = strings.ReplaceAll(text, "、、", "、")
	}

	// 文頭に残った不要な記号だけ除去。
	text = strings.TrimLeft(text, " 、。")

	return strings.TrimSpace(text)
}
func nicknameOf(
	communityID string,
	name string,
) string {
	communityID = strings.TrimSpace(communityID)
	name = strings.TrimSpace(name)

	if communityID == "" || name == "" {
		return ""
	}

	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	communityNicknames := nicknames[communityID]

	if nickname, ok := communityNicknames[name]; ok {
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
func getRandomLearnedMaterial(
	communityID string,
) string {
	learnedWordsMu.RLock()
	defer learnedWordsMu.RUnlock()

	var candidates []string
	seen := make(map[string]bool)

	// 覚えているフレーズから素材を集める。
	for _, phrase := range learnedPhrases {
		text := strings.TrimSpace(phrase.Text)

		if text == "" ||
			isProtectedName(text) ||
			isMeaninglessLearnedText(text) ||
			seen[text] {
			continue
		}

		seen[text] = true
		candidates = append(candidates, text)
	}

	// 覚えているペアからも素材を集める。
	for _, pair := range learnedPairs[communityID] {
		values := []string{
			strings.TrimSpace(pair.Key),
			strings.TrimSpace(pair.Value),
		}

		for _, text := range values {
			if text == "" ||
				isProtectedName(text) ||
				isMeaninglessLearnedText(text) ||
				seen[text] {
				continue
			}

			seen[text] = true
			candidates = append(candidates, text)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// 出現回数による重み付けはしない。
	// 覚えている素材すべてから均等にランダム選択する。
	material := candidates[rand.Intn(len(candidates))]

	slog.Info("random learned material",
		slog.String("material", material),
		slog.Int("candidates", len(candidates)),
	)

	return material
}
func getLearnedMaterials(
	communityID string,
) []string {
	learnedWordsMu.RLock()
	defer learnedWordsMu.RUnlock()

	var candidates []string
	seen := make(map[string]bool)

	for _, phrase := range learnedPhrases {
		text := strings.TrimSpace(phrase.Text)

		if text == "" ||
			isProtectedName(text) ||
			isMeaninglessLearnedText(text) ||
			seen[text] {
			continue
		}

		seen[text] = true
		candidates = append(candidates, text)
	}

	for _, pair := range learnedPairs[communityID] {
		values := []string{
			strings.TrimSpace(pair.Key),
			strings.TrimSpace(pair.Value),
		}

		for _, text := range values {
			if text == "" ||
				isProtectedName(text) ||
				isMeaninglessLearnedText(text) ||
				seen[text] {
				continue
			}

			seen[text] = true
			candidates = append(candidates, text)
		}
	}

	return candidates
}
func generateMemoryPost(
	communityID string,
) string {
	communityID = strings.TrimSpace(communityID)

	if communityID == "" {
		return ""
	}

	learnedWordsMu.RLock()

	var phraseCandidates []LearnedPhrase

	// learnedPhrases は現在ほぼ未使用だが、
	// 既存処理を壊さないため候補としては残す。
	for _, phrase := range learnedPhrases {
		if isProtectedName(phrase.Text) {
			continue
		}

		if isMeaninglessLearnedText(phrase.Text) {
			continue
		}

		phraseCandidates = append(
			phraseCandidates,
			phrase,
		)
	}

	var pairCandidates []LearnedPair

	// このコミュで学習したペアだけを見る。
	for _, pair := range learnedPairs[communityID] {
		if isProtectedName(pair.Key) ||
			isProtectedName(pair.Value) {
			continue
		}

		if isMeaninglessLearnedText(pair.Key) ||
			isMeaninglessLearnedText(pair.Value) {
			continue
		}

		pairCandidates = append(
			pairCandidates,
			pair,
		)
	}

	learnedWordsMu.RUnlock()

	// 覚えたフレーズを、そのまま使うこともある。
	if len(phraseCandidates) > 0 &&
		rand.Intn(100) < 30 {

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
				post := addEmoji(
					applyNicknames(
						communityID,
						phrase.Text,
					),
				)

				slog.Info(
					"generated learned phrase",
					slog.String("communityID", communityID),
					slog.String("post", post),
				)

				return post
			}

			pick -= weight
		}
	}

	// 覚えたペアが無ければ、覚えたフレーズを使う。
	if len(pairCandidates) == 0 {
		if len(phraseCandidates) > 0 {
			phrase := phraseCandidates[rand.Intn(len(phraseCandidates))]

			post := addEmoji(
				applyNicknames(
					communityID,
					phrase.Text,
				),
			)

			slog.Info(
				"generated learned phrase",
				slog.String("communityID", communityID),
				slog.String("post", post),
			)

			return post
		}

		return ""
	}

	// このコミュで覚えた言葉からスタート。
	pair := pairCandidates[rand.Intn(len(pairCandidates))]

	parts := []string{
		pair.Key + pair.Value,
	}

	current := pair.Value

	// 内部記憶 → 外部辞書から1～2語つなぐ。
	chainLength := 1 + rand.Intn(2)
	for i := 0; i < chainLength; i++ {
		next, ok := findNextWord(
			communityID,
			current,
		)

		if !ok || next == "" {
			break
		}

		parts = append(parts, next)
		current = next
	}

	post := shapeDaseiParts(parts)

	post = applyNicknames(
		communityID,
		post,
	)

	post = addEmoji(post)

	slog.Info(
		"generated learned post",
		slog.String("communityID", communityID),
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
func findNextWord(
	communityID string,
	word string,
) (string, bool) {
	communityID = strings.TrimSpace(communityID)
	word = strings.TrimSpace(word)

	if word == "" {
		return "", false
	}

	// まず、このコミュでだせいが覚えた言葉から探す。
	learnedWordsMu.RLock()

	candidates := []string{}

	for _, pair := range learnedPairs[communityID] {
		if pair.Key == word &&
			!isProtectedName(pair.Key) &&
			!isProtectedName(pair.Value) {

			candidates = append(
				candidates,
				pair.Value,
			)
		}
	}

	learnedWordsMu.RUnlock()

	// このコミュの内部記憶に候補があれば使う。
	if len(candidates) > 0 {
		return candidates[rand.Intn(len(candidates))], true
	}

	// 内部記憶に無ければ外部辞書へ。
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
func applyNicknames(
	communityID string,
	text string,
) string {
	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	communityNicknames := nicknames[communityID]

	for name, nickname := range communityNicknames {
		text = strings.ReplaceAll(
			text,
			name+"さん",
			nickname,
		)

		text = strings.ReplaceAll(
			text,
			name,
			nickname,
		)
	}

	return text
}
func ensureDaseiPostLength(post string, fragments []string) string {
	post = strings.TrimSpace(post)

	if post == "" {
		return ""
	}

	// 15%は短文をそのまま許容。
	if rand.Intn(100) < 15 {
		if len([]rune(post)) <= 49 {
			return post
		}
	}

	runes := []rune(post)

	// 140文字を超えていたら切る。
	if len(runes) > 140 {
		return string(runes[:140])
	}

	// 50文字以上なら、そのまま。
	if len(runes) >= 50 {
		return post
	}
	if len(fragments) == 0 {
		return post
	}

	result := post
	used := make(map[string]bool)

	for _, fragment := range fragments {
		fragment = strings.TrimSpace(fragment)

		if fragment == "" {
			continue
		}

		if used[fragment] {
			continue
		}

		next := result + " " + fragment

		if len([]rune(next)) > 140 {
			continue
		}

		result = next
		used[fragment] = true

		if len([]rune(result)) >= 50 {
			break
		}
	}

	return strings.TrimSpace(result)
}
func createMutter(
	communityID string,
	text string,
) string {
	communityID = strings.TrimSpace(communityID)

	if communityID == "" {
		return ""
	}

	// 30%は中二病。
	if rand.Intn(100) < 30 {
		post := createChuunibyou(communityID)

		if post == "" {
			return ""
		}

		post = finalizeDaseiReply(post, post)

		return decorateMutter(
			communityID,
			post,
		)
	}

	learnedMaterial := strings.TrimSpace(
		getRandomLearnedMaterial(communityID),
	)

	memoryMu.RLock()

	communityMemories := memories[communityID]

	values := make(
		[]string,
		0,
		len(communityMemories),
	)

	for name, m := range communityMemories {
		nicknameMu.RLock()

		communityNicknames := nicknames[communityID]
		_, isOriginalName := communityNicknames[name]

		nicknameMu.RUnlock()

		if isOriginalName {
			continue
		}

		value := strings.TrimSpace(m.Value)

		if value == "" {
			continue
		}

		values = append(
			values,
			value,
		)
	}

	memoryMu.RUnlock()
	if len(values) > 0 {
		material := values[rand.Intn(len(values))]

		if learnedMaterial != "" {
			material = material + " " + learnedMaterial
		}

		draft := generateDaseiDraftFromReference(
			material,
		)

		if draft != "" {
			draft = applyNicknames(
				communityID,
				draft,
			)

			draft = finalizeDaseiReply(
				material,
				draft,
			)

			return decorateMutter(
				communityID,
				draft,
			)
		}

		return decorateMutter(
			communityID,
			applyNicknames(
				communityID,
				material,
			),
		)

	}
	mutters := []string{
		"？",
		"はい",
		"たぶん",
		"えーとえーと",
	}

	return decorateMutter(
		communityID,
		applyNicknames(
			communityID,
			mutters[rand.Intn(len(mutters))],
		),
	)
}
func generateDaseiDraftFromReference(material string) string {
	material = strings.TrimSpace(material)

	if material == "" {
		return ""
	}

	wikipediaResult := searchWikipedia(material)
	networkResult := searchWeb(material)

	webResult := wikipediaResult + "\n" + networkResult

	slog.Info("Dasei draft reference search",
		slog.String("material", material),
		slog.String("wikipedia", wikipediaResult),
		slog.String("network", networkResult),
	)

	// 検索結果から「言葉」だけを拾う。
	// 文章そのものは使わない。
	referenceWords := extractReferenceWords(webResult)

	if len(referenceWords) == 0 {
		return ""
	}

	// 重複を除きながら素材を集める。
	var words []string
	seen := make(map[string]bool)

	for _, word := range referenceWords {
		word = strings.TrimSpace(word)

		if word == "" || word == material {
			continue
		}

		if len([]rune(word)) < 2 {
			continue
		}

		if seen[word] {
			continue
		}

		seen[word] = true
		words = append(words, word)
	}

	if len(words) == 0 {
		return ""
	}
	// 短い言葉を素材として混ぜる。
	// 長文テンプレではなく、あくまで文章を組み立てるための部品。
	fragments := []string{
		material,
		"なんか",
		"ちょっと",
		"気になる",
		"面白いな",
		"そうなんや",
		"らしい",
		"というか",
		"たぶん",
		"だせいは知らんかった",
		"これ意外やな",
		"そういう話なんか",
		"よく分からんけど",
		"なんとなく",
	}

	// 検索結果の言葉は毎回ランダムに絞って使う。
	// 同じ検索結果から同じ言葉ばかり出るのを防ぐ。
	rand.Shuffle(len(words), func(i, j int) {
		words[i], words[j] = words[j], words[i]
	})

	useCount := rand.Intn(5) + 3 // 3〜7語

	if useCount > len(words) {
		useCount = len(words)
	}

	for _, word := range words[:useCount] {
		fragments = append(fragments, word)
	}

	// 素材をシャッフルする。
	rand.Shuffle(len(fragments), func(i, j int) {
		fragments[i], fragments[j] = fragments[j], fragments[i]
	})

	// 基本は50〜140文字。
	// ごく一部だけ短文を許容する。
	shortDraft := rand.Intn(100) < 15

	var result []string
	used := make(map[string]bool)
	length := 0

	for _, fragment := range fragments {
		fragment = strings.TrimSpace(fragment)

		if fragment == "" {
			continue
		}

		if used[fragment] {
			continue
		}

		addLength := len([]rune(fragment))

		if length+addLength > 140 {
			continue
		}

		result = append(result, fragment)
		used[fragment] = true
		length += addLength
		// 短文はここで終了。
		if shortDraft && length >= 10 {
			break
		}

		// 通常は50文字を超えたところから、
		// ランダムに終了する。
		if !shortDraft && length >= 50 {
			if rand.Intn(100) < 35 {
				break
			}
		}
	}

	if len(result) == 0 {
		return ""
	}

	// 素材をつなぐ。
	// ここでは文章を完成させようとしない。
	draft := strings.TrimSpace(strings.Join(result, " "))

	if draft == "" {
		return ""
	}

	draft = ensureDaseiPostLength(draft, fragments)

	slog.Info("Dasei draft generated",
		slog.String("material", material),
		slog.String("draft", draft),
		slog.Int("length", len([]rune(draft))),
	)

	return draft
}
func createChuunibyou(
	communityID string,
) string {
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

	communityMemories := memories[communityID]

	values := make(
		[]string,
		0,
		len(communityMemories),
	)

	for _, m := range communityMemories {
		value := strings.TrimSpace(m.Value)

		if value == "" {
			continue
		}

		values = append(values, value)
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

	result = applyNicknames(
		communityID,
		result,
	)

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
func getMemberNickname(
	communityID string,
	userID string,
) string {
	communityID = strings.TrimSpace(communityID)
	userID = strings.TrimSpace(userID)

	if communityID == "" || userID == "" {
		return ""
	}

	membersMu.RLock()
	name := strings.TrimSpace(members[userID])
	membersMu.RUnlock()

	if name == "" {
		return ""
	}

	slog.Info(
		"nickname lookup",
		slog.String("communityID", communityID),
		slog.String("userID", userID),
		slog.String("memberName", name),
	)

	// まず、このコミュのニックネームだけを見る。
	nicknameMu.RLock()

	communityNicknames := nicknames[communityID]
	nickname := strings.TrimSpace(communityNicknames[name])

	nicknameMu.RUnlock()

	if nickname != "" {
		slog.Info(
			"nickname cache hit",
			slog.String("communityID", communityID),
			slog.String("memberName", name),
			slog.String("nickname", nickname),
		)

		return nickname
	}

	// メモリに無ければ、このコミュのDB記録から復元する。
	savedNickname, err := LoadNickname(
		communityID,
		name,
	)

	if err != nil {
		slog.Error(
			"LoadNickname failed",
			slog.String("communityID", communityID),
			slog.String("name", name),
			slog.String("error", err.Error()),
		)

		return ""
	}

	savedNickname = strings.TrimSpace(savedNickname)

	slog.Info(
		"nickname loaded",
		slog.String("communityID", communityID),
		slog.String("memberName", name),
		slog.String("nickname", savedNickname),
	)

	if savedNickname == "" {
		return ""
	}

	// 次回以降はDBを読まなくて済むよう、
	// このコミュ専用のキャッシュへ保存する。
	nicknameMu.Lock()

	if nicknames[communityID] == nil {
		nicknames[communityID] = make(map[string]string)
	}

	nicknames[communityID][name] = savedNickname

	nicknameMu.Unlock()

	return savedNickname
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
func (h *Handler) getThreadPosts(
	authCtx context.Context,
	currentPost *modelv1.Post,
) ([]*modelv1.Post, error) {
	if currentPost == nil {
		return nil, nil
	}

	var reversed []*modelv1.Post
	post := currentPost

	for post != nil {
		reversed = append(reversed, post)

		parentID := strings.TrimSpace(post.GetInReplyToPostId())
		if parentID == "" {
			break
		}

		resp, err := h.apiClient.GetPosts(
			authCtx,
			&application_apiv1.GetPostsRequest{
				PostIdList: []string{parentID},
			},
		)
		if err != nil {
			return nil, err
		}

		posts := resp.GetPosts()
		if len(posts) == 0 || posts[0] == nil {
			break
		}

		post = posts[0]

		// 念のため最大30投稿まで。
		if len(reversed) >= 30 {
			break
		}
	}

	// 「現在→親→先頭」から「先頭→現在」へ並べ直す。
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}

	return reversed, nil
}
func GenerateReplyWithThread(
	communityID string,
	text string,
	isMention bool,
	threadPosts []*modelv1.Post,
	daseiCreatorID string,
) string {
	var contextParts []string
	var humanContextParts []string
	var daseiContextParts []string

	for _, post := range threadPosts {
		if post == nil {
			continue
		}

		postText := strings.TrimSpace(post.GetText())
		if postText == "" || postText == strings.TrimSpace(text) {
			continue
		}

		contextParts = append(contextParts, postText)

		if daseiCreatorID != "" && post.GetCreatorId() == daseiCreatorID {
			daseiContextParts = append(daseiContextParts, postText)
		} else {
			humanContextParts = append(humanContextParts, postText)
		}
	}

	threadContext := strings.Join(contextParts, "\n")
	humanThreadContext := strings.Join(humanContextParts, "\n")
	daseiThreadContext := strings.Join(daseiContextParts, "\n")

	_ = humanThreadContext
	_ = daseiThreadContext
	normalized := strings.TrimSpace(text)

	if strings.Contains(normalized, "何の話") ||
		strings.Contains(normalized, "なんの話") ||
		strings.Contains(normalized, "何話してた") ||
		strings.Contains(normalized, "なんの話してた") {

		if len(contextParts) > 0 {
			for i := 0; i < len(contextParts); i++ {
				past := strings.TrimSpace(contextParts[i])

				if past == "" {
					continue
				}

				if strings.Contains(past, "何の話") ||
					strings.Contains(past, "なんの話") {
					continue
				}

				return finalizeDaseiReply(
					past,
					"さっきは「"+past+"」って話してたよ。",
				)
			}
		}
	}
	if isMention {
		if reply := mentionReply(text); reply != "" {
			return reply
		}
	}

	return createReply(
		communityID,
		text,
		threadContext,
	)
}
func GenerateReplyWithGroq(
	ctx context.Context,
	communityID string,
	text string,
	isMention bool,
	threadPosts []*modelv1.Post,
	daseiCreatorID string,
	nickname string,
) (string, bool) {

	text = strings.TrimSpace(text)

	var threadHistory []string

	for _, post := range threadPosts {
		if post == nil {
			continue
		}

		postText := strings.TrimSpace(post.GetText())
		if postText == "" || postText == text {
			continue
		}

		speaker := "人間"

		if daseiCreatorID != "" &&
			post.GetCreatorId() == daseiCreatorID {
			speaker = "だせい"
		}

		threadHistory = append(
			threadHistory,
			speaker+": "+postText,
		)
	}
	normalized := strings.TrimSpace(text)

	isNicknameTeach :=
		strings.HasPrefix(normalized, "だせい、") &&
			strings.Contains(normalized, "さんは") &&
			strings.HasSuffix(normalized, "さんだよ")

	isKnowledgeTeach :=
		strings.HasPrefix(normalized, "だせい、") &&
			!strings.Contains(normalized, "さんは") &&
			strings.Contains(normalized, "は") &&
			strings.HasSuffix(normalized, "だよ")

	if isNicknameTeach || isKnowledgeTeach {
		return createReply(
			communityID,
			text,
			strings.Join(threadHistory, "\n"),
		), false
	}

	if strings.Contains(text, "何の話") ||
		strings.Contains(text, "なんの話") ||
		strings.Contains(text, "何話してた") ||
		strings.Contains(text, "なんの話してた") {

		return GenerateReplyWithThread(
			communityID,
			text,
			isMention,
			threadPosts,
			daseiCreatorID,
		), false
	}

	roll := rand.Intn(100)

	// 20%：インプレゾンビ
	if roll < 20 {
		mode := rand.Intn(100)

		switch {
		case mode < 30:
			return finalizeDaseiReply(
				text,
				zombieEnglish(),
			), false

		case mode < 60:
			return finalizeDaseiReply(
				text,
				zombieJapanese(),
			), false

		default:
			return finalizeDaseiReply(
				text,
				zombieReply(text),
			), false
		}
	}
	// 20%：怪文書
	if roll < 40 {
		communityWords := getLearnedMaterials(communityID)

		result := makeKaibunsho(
			text,
			communityWords,
		)

		reply := limitKaibunshoLength(
			result.Text,
		)

		if strings.TrimSpace(reply) != "" {
			slog.Info(
				"reply mode kaibunsho",
				slog.String("mode", string(result.Mode)),
				slog.Int("level", result.Level),
				slog.Int("mixRate", result.MixRate),
				slog.Int("contamRate", result.ContamRate),
			)

			return reply, false
		}
	}

	// 60%：Groq
	memoryHint := getRelevantMemory(
		communityID,
		text,
	)

	slog.Info("nickname before groq",
		slog.String("nickname", nickname),
	)

	slog.Info("memory before groq",
		slog.String("memory", memoryHint),
	)

	reply, err := generateGroqReply(
		ctx,
		text,
		threadHistory,
		nickname,
		memoryHint,
	)

	if err == nil &&
		strings.TrimSpace(reply) != "" {

		slog.Info(
			"reply mode groq",
			slog.String("reply", reply),
		)

		return reply, true
	}
	// Groq失敗時は再試行せず、
	// 従来の返信処理へ退避する。
	if err != nil {
		slog.Error(
			"groq reply failed",
			slog.String("error", err.Error()),
		)
	}

	return GenerateReplyWithThread(
		communityID,
		text,
		isMention,
		threadPosts,
		daseiCreatorID,
	), false
}
func GenerateReply(
	text string,
	isMention bool,
) string {
	if isMention {
		if reply := mentionReply(text); reply != "" {
			return reply
		}
	}

	if reply := fixedReply(text); reply != "" {
		return reply
	}

	reply := generateRandomDaseiDraft(text)

	if reply != "" {
		return finalizeDaseiReply(
			text,
			reply,
		)
	}

	return finalizeDaseiReply(
		text,
		"そうなんだね",
	)
}
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
