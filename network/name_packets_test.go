package network

import (
	"encoding/binary"
	"testing"
)

func TestParseActorNameAck(t *testing.T) {
	data := make([]byte, 30)
	binary.LittleEndian.PutUint16(data[0:2], 0x0095)
	binary.LittleEndian.PutUint32(data[2:6], 0x11223344)
	copy(data[6:30], []byte("Kivutar\x00ignored"))

	ack, ok, err := ParseActorNameAck(Packet{ID: 0x0095, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if ack.ID != 0x11223344 || ack.Name != "Kivutar" {
		t.Fatalf("ack = %+v", ack)
	}
}

func TestParseActorNameAllAck(t *testing.T) {
	data := make([]byte, 102)
	binary.LittleEndian.PutUint16(data[0:2], 0x0195)
	binary.LittleEndian.PutUint32(data[2:6], 0x55667788)
	copyFixedName(data[6:30], "Alice")
	copyFixedName(data[30:54], "Party")
	copyFixedName(data[54:78], "Guild")
	copyFixedName(data[78:102], "Title")

	ack, ok, err := ParseActorNameAck(Packet{ID: 0x0195, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if ack.ID != 0x55667788 || ack.Name != "Alice" || ack.PartyName != "Party" || ack.GuildName != "Guild" || ack.Title != "Title" || !ack.HasGuildName {
		t.Fatalf("ack = %+v", ack)
	}
}

func TestParseActorNameByGID2Ack(t *testing.T) {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint16(data[0:2], 0x0AF7)
	binary.LittleEndian.PutUint32(data[4:8], 0x01020304)
	copyFixedName(data[8:32], "Npc Name")

	ack, ok, err := ParseActorNameAck(Packet{ID: 0x0AF7, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("packet was not parsed")
	}
	if ack.ID != 0x01020304 || ack.Name != "Npc Name" {
		t.Fatalf("ack = %+v", ack)
	}
}

func copyFixedName(dst []byte, value string) {
	for i := range dst {
		dst[i] = ' '
	}
	copy(dst, []byte(value))
}
