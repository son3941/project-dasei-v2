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

	if time.Since(memory.LearnedAt) > 10*24*time.Hour {
		memoryMu.RUnlock()

		memoryMu.Lock()
		delete(memories, matchedKey)
		memoryMu.Unlock()

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
