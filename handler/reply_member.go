package handler

import (
	"math/rand"
	"strings"
)

func randomMember(
	communityID string,
) string {
	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	communityNicknames := nicknames[communityID]

	if len(communityNicknames) == 0 {
		return ""
	}

	names := make(
		[]string,
		0,
		len(communityNicknames),
	)

	for _, nickname := range communityNicknames {
		nickname = strings.TrimSpace(nickname)

		if nickname == "" {
			continue
		}

		names = append(names, nickname)
	}

	if len(names) == 0 {
		return ""
	}

	return names[rand.Intn(len(names))]
}
