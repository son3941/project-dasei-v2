package handler

import (
	"log/slog"
	"math/rand"
	"strings"
	"time"
)

func replyFromMemory(
	communityID string,
	text string,
) string {
	communityID = strings.TrimSpace(communityID)
	text = strings.TrimSpace(text)

	if communityID == "" || text == "" {
		return ""
	}

	memoryMu.RLock()

	communityMemories := memories[communityID]

	var memory Memory
	var matchedKey string
	ok := false

	for key, m := range communityMemories {
		if !strings.Contains(text, key) {
			continue
		}

		if !m.LastReplyAt.IsZero() &&
			time.Since(m.LastReplyAt) < 5*time.Minute {
			continue
		}

		memory = m
		matchedKey = key
		ok = true
		break
	}

	memoryMu.RUnlock()

	if !ok {
		return ""
	}

	// このコミュの記憶だけ使用時刻を更新する。
	memoryMu.Lock()

	if memories[communityID] == nil {
		memories[communityID] = make(map[string]Memory)
	}

	memory.LastReplyAt = time.Now()
	memories[communityID][matchedKey] = memory

	memoryMu.Unlock()

	if rand.Intn(100) < 30 {
		name := randomMember(communityID)
		if name != "" {
			return memberPhrase(
				communityID,
				name,
			)
		}
	}

	reply := memory.Value

	// 記憶同士がつながっている場合も、
	// 同じコミュの記憶だけを使う。
	memoryMu.RLock()

	if next, exists := memories[communityID][reply]; exists {
		reply += next.Value

		if next2, exists := memories[communityID][next.Value]; exists {
			reply += next2.Value
		}
	}

	memoryMu.RUnlock()

	slog.Info(
		"replyFromMemory hit",
		slog.String("communityID", communityID),
		slog.String("key", matchedKey),
		slog.String("value", reply),
		slog.String("original", text),
	)

	return addEmoji(reply)
}
func getRelevantMemory(
	communityID string,
	text string,
) string {
	communityID = strings.TrimSpace(communityID)
	text = strings.TrimSpace(text)

	if communityID == "" || text == "" {
		return ""
	}

	memoryMu.RLock()

	communityMemories := memories[communityID]

	var memory Memory
	var matchedKey string
	found := false

	for key, m := range communityMemories {
		key = strings.TrimSpace(key)

		if key == "" ||
			!strings.Contains(text, key) {
			continue
		}

		// 同じ記憶を短時間に何度も使わない。
		if !m.LastReplyAt.IsZero() &&
			time.Since(m.LastReplyAt) < 5*time.Minute {
			continue
		}

		memory = m
		matchedKey = key
		found = true
		break
	}

	memoryMu.RUnlock()

	if !found {
		return ""
	}

	value := strings.TrimSpace(memory.Value)
	if value == "" {
		return ""
	}

	// 使用時刻を、このコミュの記憶だけ更新する。
	memoryMu.Lock()

	if memories[communityID] == nil {
		memories[communityID] = make(map[string]Memory)
	}

	memory.LastReplyAt = time.Now()
	memories[communityID][matchedKey] = memory

	memoryMu.Unlock()

	slog.Info(
		"relevant memory found",
		slog.String("communityID", communityID),
		slog.String("key", matchedKey),
		slog.String("value", value),
	)

	return matchedKey + "＝" + value
}
