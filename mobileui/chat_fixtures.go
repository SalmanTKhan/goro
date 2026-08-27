package mobileui

import "fmt"

func FixtureChat(name string) MobileChatModel {
	model := MobileChatModel{Open: true, Channel: "WORLD", CanSend: true}
	switch name {
	case "chat-empty":
		model.Messages = []MobileChatMessage{{Sender: "SYSTEM", Text: "No messages yet."}}
	case "chat-long":
		for i := 0; i < 36; i++ {
			model.Messages = append(model.Messages, MobileChatMessage{Sender: fmt.Sprintf("Player%02d", i%7+1), Text: fmt.Sprintf("Deterministic preview message %02d for scrolling and wrapping.", i+1)})
		}
	default:
		model.Messages = []MobileChatMessage{
			{Sender: "SYSTEM", Text: "Chat is ready."},
			{Sender: "Goro", Text: "Welcome to the WORLD channel."},
		}
	}
	return model
}
