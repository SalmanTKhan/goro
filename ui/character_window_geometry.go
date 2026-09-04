package ui

// The character windows are authored at a fixed desktop size. The Android
// mobile presentation runs those same windows and scales the rendered result to
// the phone, so it needs their authored extent and the margin the window
// placement leaves around them.
//
// These accessors exist so the host reads the real constants instead of
// repeating them; a change to the window's size cannot drift out of sync with
// the surface it is rasterized onto.

// CharacterWindowScreenMargin is the gap window placement keeps between a
// window and the edge of the screen.
func CharacterWindowScreenMargin() int { return windowScreenMargin }

// CharacterSelectWindowSize reports the authored size of the character select
// window.
func CharacterSelectWindowSize() (width, height int) {
	return characterSelectWindowW, characterSelectWindowH
}

// CharacterCreateWindowSize reports the authored size of the character create
// window.
func CharacterCreateWindowSize() (width, height int) {
	return characterCreateWindowW, characterCreateWindowH
}
