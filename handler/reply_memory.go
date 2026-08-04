package handler

import (
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

	return addEmoji(memory.Value)
}
