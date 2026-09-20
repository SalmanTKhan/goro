package mobileui

import "testing"

func TestOnlineLoginServerLayoutIsTouchSafeInLandscape(t *testing.T) {
	model := MobileOnlineLoginModel{
		Phase: OnlineLoginServer,
		Servers: []OnlineServerOption{
			{Index: 0, Name: "Goro Dev Server", Detail: "dev"},
			{Index: 1, Name: "KakiaRO2 Server", Detail: "secondary"},
			{Index: 2, Name: "KakiaRO Server", Detail: "legacy"},
		},
	}
	layout := LayoutOnlineLogin(FoldOuterViewport(), model)
	if len(layout.Options) != len(model.Servers) {
		t.Fatalf("server rows = %d, want %d", len(layout.Options), len(model.Servers))
	}
	for i, row := range layout.Options {
		if row.H < DefaultTokens().MinTouchTarget {
			t.Fatalf("server row %d is not touch-safe: %+v", i, row)
		}
		if row.X < layout.Safe.X || row.Y < layout.Safe.Y || row.Right() > layout.Safe.Right() || row.Bottom() > layout.Safe.Bottom() {
			t.Fatalf("server row %d escapes safe area: %+v in %+v", i, row, layout.Safe)
		}
	}
}

func TestOnlineLoginCredentialsRelayoutAcrossRotation(t *testing.T) {
	model := MobileOnlineLoginModel{Phase: OnlineLoginCredentials, Username: "player", PasswordSet: true}
	landscape := LayoutOnlineLogin(Viewport{Width: 2289, Height: 840, SafeRight: 80}, model)
	portrait := LayoutOnlineLogin(Viewport{Width: 840, Height: 2289, SafeBottom: 80}, model)

	for name, layout := range map[string]OnlineLoginLayout{"landscape": landscape, "portrait": portrait} {
		for fieldName, rect := range map[string]Rect{
			"username": layout.Username,
			"password": layout.Password,
			"submit": layout.Submit,
		} {
			if rect.W <= 0 || rect.H < DefaultTokens().MinTouchTarget {
				t.Fatalf("%s %s field is not touch-safe: %+v", name, fieldName, rect)
			}
			if rect.X < layout.Safe.X || rect.Y < layout.Safe.Y || rect.Right() > layout.Safe.Right() || rect.Bottom() > layout.Safe.Bottom() {
				t.Fatalf("%s %s field escapes safe area: %+v in %+v", name, fieldName, rect, layout.Safe)
			}
		}
	}
	if landscape.Panel == portrait.Panel {
		t.Fatal("rotation did not produce a new online-login layout")
	}
}

func TestOnlineLoginCharacterServiceUsesServerRows(t *testing.T) {
	model := MobileOnlineLoginModel{
		Phase: OnlineLoginCharacterService,
		Servers: []OnlineServerOption{{Index: 0, Name: "Chaos", UserCount: 42}},
	}
	layout := LayoutOnlineLogin(FoldOuterViewport(), model)
	if len(layout.Options) != 1 {
		t.Fatalf("character-service rows = %d, want 1", len(layout.Options))
	}
	if layout.Username.W != 0 || layout.Password.W != 0 {
		t.Fatalf("character-service layout leaked credential fields: user=%+v pass=%+v", layout.Username, layout.Password)
	}
}
