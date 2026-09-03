package network

import (
	"encoding/binary"
	"testing"
)

func TestParseAccountAcceptLogin(t *testing.T) {
	data := make([]byte, 47+32)
	binary.LittleEndian.PutUint16(data[0:2], 0x0069)
	binary.LittleEndian.PutUint16(data[2:4], uint16(len(data)))
	binary.LittleEndian.PutUint32(data[4:8], 100)
	binary.LittleEndian.PutUint32(data[8:12], 200)
	binary.LittleEndian.PutUint32(data[12:16], 5)
	copy(data[20:46], []byte("2026-06-21 15:00:00"))
	data[46] = 11

	base := 47
	copy(data[base:base+4], []byte{127, 0, 0, 1})
	binary.LittleEndian.PutUint16(data[base+4:base+6], 6121)
	copy(data[base+6:base+26], []byte("Char Server"))
	binary.LittleEndian.PutUint16(data[base+26:base+28], 42)

	parsed, err := ParseAccountAcceptLogin(Packet{ID: 0x0069, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.AuthCode != 100 || parsed.AccountID != 200 || parsed.UserLevel != 5 || parsed.Sex != 1 {
		t.Fatalf("unexpected header: %+v", parsed)
	}
	if len(parsed.CharServer) != 1 {
		t.Fatalf("servers = %d", len(parsed.CharServer))
	}
	server := parsed.CharServer[0]
	if server.Address != "127.0.0.1" || server.Port != 6121 || server.Name != "Char Server" || server.UserCount != 42 {
		t.Fatalf("unexpected server: %+v", server)
	}
}

func TestParseAccountRefuseLogin(t *testing.T) {
	data := make([]byte, 23)
	binary.LittleEndian.PutUint16(data[0:2], 0x006A)
	data[2] = 6
	copy(data[3:23], []byte("2026-08-29 21:40:00"))

	parsed, err := ParseAccountRefuseLogin(Packet{ID: 0x006A, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ErrorCode != 6 || parsed.UnblockTime != "2026-08-29 21:40:00" {
		t.Fatalf("unexpected refusal: %+v", parsed)
	}
}

func TestParseAccountRefuseLoginRejectsShortPacket(t *testing.T) {
	_, err := ParseAccountRefuseLogin(Packet{ID: 0x006A, Data: make([]byte, 22)})
	if err == nil {
		t.Fatal("short AC_REFUSE_LOGIN packet was accepted")
	}
}

func TestParseCharListLegacy108(t *testing.T) {
	data := make([]byte, 24+108)
	binary.LittleEndian.PutUint16(data[0:2], 0x006B)
	binary.LittleEndian.PutUint16(data[2:4], uint16(len(data)))
	char := data[24:]
	binary.LittleEndian.PutUint32(char[0:4], 1234)
	binary.LittleEndian.PutUint32(char[4:8], 123456)
	binary.LittleEndian.PutUint32(char[8:12], 95000)
	binary.LittleEndian.PutUint32(char[16:20], 9)
	binary.LittleEndian.PutUint32(char[28:32], 8)
	binary.LittleEndian.PutUint16(char[42:44], 70)
	binary.LittleEndian.PutUint16(char[44:46], 100)
	binary.LittleEndian.PutUint16(char[52:54], 7)
	binary.LittleEndian.PutUint16(char[56:58], 1201)
	binary.LittleEndian.PutUint16(char[58:60], 42)
	binary.LittleEndian.PutUint16(char[62:64], 11)
	binary.LittleEndian.PutUint16(char[64:66], 2101)
	binary.LittleEndian.PutUint16(char[66:68], 22)
	binary.LittleEndian.PutUint16(char[68:70], 33)
	binary.LittleEndian.PutUint16(char[70:72], 5)
	binary.LittleEndian.PutUint16(char[72:74], 6)
	copy(char[74:98], []byte("Alice"))
	char[98] = 9
	char[104] = 2
	char[105] = 5

	parsed, err := ParseCharList(Packet{ID: 0x006B, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Layout != "legacy_24_108" {
		t.Fatalf("layout = %s", parsed.Layout)
	}
	if len(parsed.Characters) != 1 {
		t.Fatalf("characters = %d", len(parsed.Characters))
	}
	got := parsed.Characters[0]
	if got.ID != 1234 || got.Exp != 123456 || got.Money != 95000 || got.Name != "Alice" || got.HP != 70 || got.MaxHP != 100 || got.Job != 7 || got.Level != 42 || got.JobLevel != 9 || got.Str != 9 || got.Slot != 2 || got.HairColor != 5 || got.HeadPal != 5 || got.BodyPal != 6 || got.Weapon != 1201 || got.Shield != 2101 || got.HeadLow != 11 || got.HeadTop != 22 || got.HeadMid != 33 || got.Option != 8 {
		t.Fatalf("unexpected character: %+v", got)
	}
}

func TestParseMakeCharacterAccept(t *testing.T) {
	data := make([]byte, 110)
	binary.LittleEndian.PutUint16(data[0:2], 0x006D)
	char := data[2:]
	binary.LittleEndian.PutUint32(char[0:4], 4321)
	binary.LittleEndian.PutUint16(char[52:54], 0)
	binary.LittleEndian.PutUint16(char[54:56], 9)
	binary.LittleEndian.PutUint16(char[58:60], 1)
	copy(char[74:98], []byte("Newbie"))
	char[98] = 5
	char[99] = 5
	char[100] = 5
	char[101] = 5
	char[102] = 5
	char[103] = 5
	char[104] = 3
	char[105] = 8

	got, err := ParseMakeCharacterAccept(Packet{ID: 0x006D, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 4321 || got.Name != "Newbie" || got.Slot != 3 || got.Hair != 9 || got.HairColor != 8 || got.Str != 5 {
		t.Fatalf("created character = %+v", got)
	}
}

func TestParseMakeCharacterRefuse(t *testing.T) {
	code, err := ParseMakeCharacterRefuse(Packet{ID: 0x006E, Data: []byte{0x6e, 0x00, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestParseDeleteCharacterResponses(t *testing.T) {
	if err := ParseDeleteCharacterAccept(Packet{ID: 0x006F, Data: []byte{0x6f, 0x00}}); err != nil {
		t.Fatalf("accept err = %v", err)
	}
	code, err := ParseDeleteCharacterRefuse(Packet{ID: 0x0070, Data: []byte{0x70, 0x00, 4}})
	if err != nil {
		t.Fatalf("refuse err = %v", err)
	}
	if code != 4 {
		t.Fatalf("code = %d, want 4", code)
	}
}

func TestParseZoneServerNotify(t *testing.T) {
	data := make([]byte, 28)
	binary.LittleEndian.PutUint16(data[0:2], 0x0071)
	binary.LittleEndian.PutUint32(data[2:6], 1234)
	copy(data[6:22], []byte("prontera.gat"))
	copy(data[22:26], []byte{127, 0, 0, 1})
	binary.LittleEndian.PutUint16(data[26:28], 5121)

	zone, err := ParseZoneServerNotify(Packet{ID: 0x0071, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if zone.CharID != 1234 || zone.MapName != "prontera" || zone.Address != "127.0.0.1" || zone.Port != 5121 {
		t.Fatalf("unexpected zone: %+v", zone)
	}
}

func TestParseMapAcceptEnter(t *testing.T) {
	data := make([]byte, 11)
	binary.LittleEndian.PutUint16(data[0:2], 0x0073)
	binary.LittleEndian.PutUint32(data[2:6], 123)
	data[6], data[7], data[8] = packPosition(150, 200, 3)

	enter, err := ParseMapAcceptEnter(Packet{ID: 0x0073, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if enter.ServerTick != 123 || enter.X != 150 || enter.Y != 200 || enter.Dir != 3 {
		t.Fatalf("unexpected enter: %+v", enter)
	}
}

func TestParseMapChangeSameServer(t *testing.T) {
	data := make([]byte, 22)
	binary.LittleEndian.PutUint16(data[0:2], 0x0091)
	copy(data[2:18], []byte("geffen.gat"))
	binary.LittleEndian.PutUint16(data[18:20], 120)
	binary.LittleEndian.PutUint16(data[20:22], 80)

	change, ok, err := ParseMapChange(Packet{ID: 0x0091, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if change.MapName != "geffen" || change.X != 120 || change.Y != 80 || change.ServerMove {
		t.Fatalf("unexpected map change: %+v", change)
	}
}

func TestParseMapChangeServerMove(t *testing.T) {
	data := make([]byte, 28)
	binary.LittleEndian.PutUint16(data[0:2], 0x0092)
	copy(data[2:18], []byte("izlude.rsw"))
	binary.LittleEndian.PutUint16(data[18:20], 33)
	binary.LittleEndian.PutUint16(data[20:22], 44)
	copy(data[22:26], []byte{127, 0, 0, 1})
	binary.LittleEndian.PutUint16(data[26:28], 5121)

	change, ok, err := ParseMapChange(Packet{ID: 0x0092, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if change.MapName != "izlude" || change.X != 33 || change.Y != 44 || change.Address != "127.0.0.1" || change.Port != 5121 || !change.ServerMove {
		t.Fatalf("unexpected map change: %+v", change)
	}
}

func TestParseMapChangeServerMoveDomain(t *testing.T) {
	data := make([]byte, 156)
	binary.LittleEndian.PutUint16(data[0:2], 0x0AC7)
	copy(data[2:18], []byte("izlude_in.gat"))
	binary.LittleEndian.PutUint16(data[18:20], 65)
	binary.LittleEndian.PutUint16(data[20:22], 87)
	copy(data[22:26], []byte{127, 0, 0, 1})
	binary.LittleEndian.PutUint16(data[26:28], 5121)
	copy(data[28:], []byte("localhost"))

	change, ok, err := ParseMapChange(Packet{ID: 0x0AC7, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if change.MapName != "izlude_in" || change.X != 65 || change.Y != 87 || change.Address != "127.0.0.1" || change.Port != 5121 || !change.ServerMove {
		t.Fatalf("unexpected map change: %+v", change)
	}
}

func TestParseMapCellUpdate(t *testing.T) {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint16(data[0:2], 0x0192)
	binary.LittleEndian.PutUint16(data[2:4], 123)
	binary.LittleEndian.PutUint16(data[4:6], 456)
	binary.LittleEndian.PutUint16(data[6:8], 5)
	copy(data[8:24], []byte("geffen.gat"))

	update, ok, err := ParseMapCellUpdate(Packet{ID: 0x0192, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if update.MapName != "geffen" || update.X != 123 || update.Y != 456 || update.RawType != 5 {
		t.Fatalf("unexpected map cell update: %+v", update)
	}
}

func packPosition(x, y, dir int) (byte, byte, byte) {
	return byte(x >> 2), byte(((x & 0x03) << 6) | ((y >> 4) & 0x3f)), byte(((y & 0x0f) << 4) | (dir & 0x0f))
}
