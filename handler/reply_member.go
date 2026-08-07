package handler

import "math/rand"

func randomMember() string {
	membersMu.RLock()
	defer membersMu.RUnlock()

	if len(members) == 0 {
		return ""
	}

	names := make([]string, 0, len(members))
	for _, name := range members {
		names = append(names, name)
	}

	return names[rand.Intn(len(names))]
}
