package handler

import (
	"math/rand"
	"strings"
)

func randomMember() string {
	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	if len(nicknames) == 0 {
		return ""
	}

	names := make([]string, 0, len(nicknames))

	for _, nickname := range nicknames {
		if strings.TrimSpace(nickname) == "" {
			continue
		}

		names = append(names, nickname)
	}

	if len(names) == 0 {
		return ""
	}

	return names[rand.Intn(len(names))]
}
