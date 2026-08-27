package game

import (
	"strings"

	"github.com/kivutar/goro/mobileui"
)

// MobileChatModel projects the existing world console into the renderer-free
// mobile surface. The console remains the online message owner; this method
// only translates its stable text representation into mobile presentation
// records.
func (m *WorldMode) MobileChatModel() mobileui.MobileChatModel {
	model := mobileui.MobileChatModel{Channel: "WORLD"}
	if m == nil {
		return model
	}
	for _, message := range m.ui.console.Messages() {
		text := strings.TrimSpace(message.Text)
		if text == "" {
			continue
		}
		sender := "SYSTEM"
		if separator := strings.Index(text, " : "); separator > 0 {
			sender = strings.TrimSpace(text[:separator])
			text = strings.TrimSpace(text[separator+3:])
		}
		model.Messages = append(model.Messages, mobileui.MobileChatMessage{Sender: sender, Text: text})
	}
	return model
}
