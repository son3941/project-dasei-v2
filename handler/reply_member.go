package handler

import "math/rand"

func randomMember() string {
	membersMu.RLock()
	defer membersMu.RUnlock()

	if len(members) == 0 {
		return ""
	}

	return members[rand.Intn(len(members))]
}
