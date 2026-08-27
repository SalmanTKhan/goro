package game

import "testing"

func TestMobileChatModelProjectsWorldConsoleMessages(t *testing.T) {
	mode := &WorldMode{}
	mode.ui.console.AddMessage("Alice : hello from world")
	mode.ui.console.AddSystemMessage("A Poring appeared.")

	model := mode.MobileChatModel()
	if model.Channel != "WORLD" || len(model.Messages) != 2 {
		t.Fatalf("model=%+v", model)
	}
	if model.Messages[0].Sender != "Alice" || model.Messages[0].Text != "hello from world" {
		t.Fatalf("chat message=%+v", model.Messages[0])
	}
	if model.Messages[1].Sender != "SYSTEM" || model.Messages[1].Text != "A Poring appeared." {
		t.Fatalf("system message=%+v", model.Messages[1])
	}
}
