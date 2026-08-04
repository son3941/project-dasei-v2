package handler

import "math/rand"

func decorateMutter(post string) string {
	if post == "" {
		return ""
	}

	post = decorateMember(post)
	post = decorateCrazy(post)
	post = decorateEmoji(post)

	return post
}

func decorateMember(post string) string {

	if rand.Intn(100) >= 20 {
		return post
	}

	name := randomMember()
	if name == "" {
		return post
	}

	if rand.Intn(2) == 0 {
		return name + "\n\n" + post
	}

	return post + "\n\n" + name
}

func decorateCrazy(post string) string {
	return createCrazyMutter(post)
}

func decorateEmoji(post string) string {
	return post
}
