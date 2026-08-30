package handler

import "math/rand"

func decorateMutter(
	communityID string,
	post string,
) string {
	if post == "" {
		return ""
	}

	post = decorateMember(
		communityID,
		post,
	)
	post = decorateCrazy(
		communityID,
		post,
	)
	post = decorateEmoji(post)

	return post
}

func decorateMember(
	communityID string,
	post string,
) string {
	if rand.Intn(100) >= 20 {
		return post
	}

	name := randomMember(
		communityID,
	)
	if name == "" {
		return post
	}

	if rand.Intn(2) == 0 {
		return name + "\n\n" + post
	}

	return post + "\n\n" + name
}

func decorateCrazy(
	communityID string,
	post string,
) string {
	return createCrazyMutter(
		communityID,
		post,
	)
}

func decorateEmoji(post string) string {
	return post
}
