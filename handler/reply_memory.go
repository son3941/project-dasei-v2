package handler

import (
	"log/slog"
	"math/rand"
	"strings"
	"time"
)

func replyFromMemory(text string) string {
	memoryMu.RLock()

	var memory Memory
	var matchedKey string
	ok := false

	for key, m := range memories {
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

	if !ok {
		memoryMu.RUnlock()
		return ""
	}

	memoryMu.RUnlock()

	memoryMu.Lock()
	memory.LastReplyAt = time.Now()
	memories[matchedKey] = memory
	memoryMu.Unlock()

	if rand.Intn(100) < 30 {
		name := randomMember()
		if name != "" {
			return memberPhrase(name)
		}
	}

	reply := memory.Value

	memoryMu.RLock()
	if next, ok := memories[reply]; ok {
		reply += next.Value

		if next2, ok := memories[next.Value]; ok {
			reply += next2.Value
		}
	}
	memoryMu.RUnlock()
	slog.Info("replyFromMemory hit",
		slog.String("key", matchedKey),
		slog.String("value", reply),
		slog.String("original", text),
	)
	return addEmoji(reply)
}
func getRelevantMemory(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	memoryMu.RLock()

	var memory Memory
	var matchedKey string
	found := false

	for key, m := range memories {
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

	// 使用時刻だけ更新する。
	memoryMu.Lock()
	memory.LastReplyAt = time.Now()
	memories[matchedKey] = memory
	memoryMu.Unlock()

	slog.Info("relevant memory found",
		slog.String("key", matchedKey),
		slog.String("value", value),
	)

	return matchedKey + "＝" + value
}
