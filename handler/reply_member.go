package handler

import "math/rand"

func randomMember() string {
	membersMu.RLock()
	defer membersMu.RUnlock()

	nicknameMu.RLock()
	defer nicknameMu.RUnlock()

	var names []string

	for _, name := range members {
		nickname, ok := nicknames[name]
		if !ok || nickname == "" {
			continue
		}

		names = append(names, nickname)
	}

	if len(names) == 0 {
		return ""
	}

	return names[rand.Intn(len(names))]
}
