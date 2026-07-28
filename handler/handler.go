package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
)

// Handler implements event.EventHandler interface.
type Handler struct {
	logger        *slog.Logger
	apiClient     application_apiv1.ApplicationServiceClient
	authenticator auth.Authenticator
}

// NewHandler creates a new Handler.
func NewHandler(apiClient application_apiv1.ApplicationServiceClient, authenticator auth.Authenticator) *Handler {
	return &Handler{
		logger:        slog.Default(),
		apiClient:     apiClient,
		authenticator: authenticator,
	}
}

// Handle processes events from mixi2.
func (h *Handler) Handle(ctx context.Context, ev *modelv1.Event) error {
	switch ev.EventType {
	case constv1.EventType_EVENT_TYPE_POST_CREATED:
		h.logger.Info("received POST_CREATED event",
			slog.String("event_id", ev.EventId),
		)
		// Add your post created event handling logic here
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
func createReply(text string) string {

	switch {

	case strings.Contains(text, "こんにちは"):
		return "こんにちはなのだ！"

	case strings.Contains(text, "おはよう"):
		return "おはようなのだ！"

	case strings.Contains(text, "こんばんは"):
		return "こんばんはなのだ！"

	case strings.Contains(text, "おやすみ"):
		return "またあとでなのだ！"

	case strings.Contains(text, "疲れた"):
		return "無理しなくていいのだ。"

	case strings.Contains(text, "眠い"):
		return "だせいも眠いのだ。"

	case strings.Contains(text, "カレー"):
		return "カレーは飲み物なのだ。"

	case strings.Contains(text, "かわいい"):
		return "照れるのだ。"

	case strings.Contains(text, "だせい"):
		return "呼んだのだ？"

	default:
		return "なるほどなのだ。"
	}
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

	reply := createReply(userText)

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
