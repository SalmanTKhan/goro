package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

type AccountAcceptLogin struct {
	AuthCode   uint32
	AccountID  uint32
	UserLevel  uint32
	LastLogin  string
	Sex        uint8
	CharServer []CharServer
}

type AccountRefuseLogin struct {
	ErrorCode   uint8
	UnblockTime string
}

type Character struct {
	ID        uint32
	Exp       int64
	Money     int64
	Name      string
	Slot      uint8
	Level     int16
	JobLevel  int16
	Job       int16
	HP        int16
	MaxHP     int16
	SP        int16
	MaxSP     int16
	Str       uint8
	Agi       uint8
	Vit       uint8
	Int       uint8
	Dex       uint8
	Luk       uint8
	Hair      int16
	HairColor uint8
	HeadPal   int16
	BodyPal   int16
	Weapon    int16
	Shield    int16
	HeadTop   int16
	HeadMid   int16
	HeadLow   int16
	Option    uint32
}

type CharList struct {
	Layout     string
	Characters []Character
}

type ZoneServerNotify struct {
	CharID  uint32
	MapName string
	Address string
	Port    uint16
}

type MapAcceptEnter struct {
	ServerTick uint32
	X          int
	Y          int
	Dir        int
}

type CharServer struct {
	Address   string
	Port      uint16
	Name      string
	UserCount uint16
	State     uint16
	Property  uint16
}

func ParseAccountAcceptLogin(packet Packet) (AccountAcceptLogin, error) {
	if packet.ID != 0x0069 {
		return AccountAcceptLogin{}, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 47 {
		return AccountAcceptLogin{}, fmt.Errorf("AC_ACCEPT_LOGIN too short: %d", len(packet.Data))
	}

	packetLen := int(binary.LittleEndian.Uint16(packet.Data[2:4]))
	if packetLen > len(packet.Data) {
		return AccountAcceptLogin{}, fmt.Errorf("AC_ACCEPT_LOGIN length %d exceeds data %d", packetLen, len(packet.Data))
	}

	sex := packet.Data[46]
	if sex >= 10 {
		sex -= 10
	}

	out := AccountAcceptLogin{
		AuthCode:  binary.LittleEndian.Uint32(packet.Data[4:8]),
		AccountID: binary.LittleEndian.Uint32(packet.Data[8:12]),
		UserLevel: binary.LittleEndian.Uint32(packet.Data[12:16]),
		LastLogin: fixedString(packet.Data[20:46]),
		Sex:       sex,
	}

	serverBytes := packetLen - 47
	if serverBytes < 0 {
		serverBytes = 0
	}
	serverCount := serverBytes / 32
	for i := 0; i < serverCount; i++ {
		base := 47 + i*32
		raw := packet.Data[base : base+32]
		ip := net.IPv4(raw[0], raw[1], raw[2], raw[3])
		out.CharServer = append(out.CharServer, CharServer{
			Address:   ip.String(),
			Port:      binary.LittleEndian.Uint16(raw[4:6]),
			Name:      fixedString(raw[6:26]),
			UserCount: binary.LittleEndian.Uint16(raw[26:28]),
			State:     binary.LittleEndian.Uint16(raw[28:30]),
			Property:  binary.LittleEndian.Uint16(raw[30:32]),
		})
	}

	return out, nil
}

func ParseAccountRefuseLogin(packet Packet) (AccountRefuseLogin, error) {
	if packet.ID != 0x006A {
		return AccountRefuseLogin{}, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 23 {
		return AccountRefuseLogin{}, fmt.Errorf("AC_REFUSE_LOGIN too short: %d", len(packet.Data))
	}

	return AccountRefuseLogin{
		ErrorCode:   packet.Data[2],
		UnblockTime: fixedString(packet.Data[3:23]),
	}, nil
}

func ParseCharList(packet Packet) (CharList, error) {
	if packet.ID != 0x006B {
		return CharList{}, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 4 {
		return CharList{}, fmt.Errorf("HC_ACCEPT_ENTER too short: %d", len(packet.Data))
	}
	packetLen := int(binary.LittleEndian.Uint16(packet.Data[2:4]))
	if packetLen > len(packet.Data) {
		return CharList{}, fmt.Errorf("HC_ACCEPT_ENTER length %d exceeds data %d", packetLen, len(packet.Data))
	}

	layout, headerSize, recordSize, err := resolveCharListLayout(packetLen)
	if err != nil {
		return CharList{}, err
	}

	out := CharList{Layout: layout}
	for offset := headerSize; offset+recordSize <= packetLen; offset += recordSize {
		var character Character
		if recordSize == 106 {
			character = parseLegacyCharacter106(packet.Data[offset : offset+recordSize])
		} else {
			character = parseCharacter108(packet.Data[offset : offset+recordSize])
		}
		out.Characters = append(out.Characters, character)
	}
	return out, nil
}

func ParseMakeCharacterAccept(packet Packet) (Character, error) {
	if packet.ID != 0x006D {
		return Character{}, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 110 {
		return Character{}, fmt.Errorf("HC_ACCEPT_MAKECHAR too short: %d", len(packet.Data))
	}
	return parseCharacter108(packet.Data[2:110]), nil
}

func ParseMakeCharacterRefuse(packet Packet) (uint8, error) {
	if packet.ID != 0x006E {
		return 0, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 3 {
		return 0, fmt.Errorf("HC_REFUSE_MAKECHAR too short: %d", len(packet.Data))
	}
	return packet.Data[2], nil
}

func ParseDeleteCharacterAccept(packet Packet) error {
	if packet.ID != 0x006F {
		return fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 2 {
		return fmt.Errorf("HC_ACCEPT_DELETECHAR too short: %d", len(packet.Data))
	}
	return nil
}

func ParseDeleteCharacterRefuse(packet Packet) (uint8, error) {
	if packet.ID != 0x0070 {
		return 0, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 3 {
		return 0, fmt.Errorf("HC_REFUSE_DELETECHAR too short: %d", len(packet.Data))
	}
	return packet.Data[2], nil
}

func ParseZoneServerNotify(packet Packet) (ZoneServerNotify, error) {
	if packet.ID != 0x0071 {
		return ZoneServerNotify{}, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 28 {
		return ZoneServerNotify{}, fmt.Errorf("HC_NOTIFY_ZONESVR too short: %d", len(packet.Data))
	}

	mapName := normalizeMapName(fixedString(packet.Data[6:22]))

	return ZoneServerNotify{
		CharID:  binary.LittleEndian.Uint32(packet.Data[2:6]),
		MapName: mapName,
		Address: net.IPv4(packet.Data[22], packet.Data[23], packet.Data[24], packet.Data[25]).String(),
		Port:    binary.LittleEndian.Uint16(packet.Data[26:28]),
	}, nil
}

func ParseMapAcceptEnter(packet Packet) (MapAcceptEnter, error) {
	if packet.ID != 0x0073 && packet.ID != 0x02EB {
		return MapAcceptEnter{}, fmt.Errorf("unexpected packet 0x%04X", packet.ID)
	}
	if len(packet.Data) < 9 {
		return MapAcceptEnter{}, fmt.Errorf("ZC_ACCEPT_ENTER too short: %d", len(packet.Data))
	}

	return MapAcceptEnter{
		ServerTick: binary.LittleEndian.Uint32(packet.Data[2:6]),
		X:          (int(packet.Data[6]) << 2) | int(packet.Data[7]>>6),
		Y:          (int(packet.Data[7]&0x3f) << 4) | int(packet.Data[8]>>4),
		Dir:        int(packet.Data[8] & 0x0f),
	}, nil
}

func resolveCharListLayout(packetLen int) (layout string, headerSize int, recordSize int, err error) {
	type candidate struct {
		name   string
		header int
		record int
	}
	for _, c := range []candidate{
		{name: "expanded_27_108", header: 27, record: 108},
		{name: "legacy_24_108", header: 24, record: 108},
		{name: "legacy_4_106", header: 4, record: 106},
	} {
		if packetLen >= c.header && (packetLen-c.header)%c.record == 0 {
			return c.name, c.header, c.record, nil
		}
	}
	return "", 0, 0, fmt.Errorf("unsupported HC_ACCEPT_ENTER layout length=%d", packetLen)
}

func parseCharacter108(data []byte) Character {
	return Character{
		ID:        binary.LittleEndian.Uint32(data[0:4]),
		Exp:       int64(int32(binary.LittleEndian.Uint32(data[4:8]))),
		Money:     int64(int32(binary.LittleEndian.Uint32(data[8:12]))),
		JobLevel:  int16(binary.LittleEndian.Uint32(data[16:20])),
		HP:        int16(binary.LittleEndian.Uint16(data[42:44])),
		MaxHP:     int16(binary.LittleEndian.Uint16(data[44:46])),
		SP:        int16(binary.LittleEndian.Uint16(data[46:48])),
		MaxSP:     int16(binary.LittleEndian.Uint16(data[48:50])),
		Option:    binary.LittleEndian.Uint32(data[28:32]),
		Job:       int16(binary.LittleEndian.Uint16(data[52:54])),
		Hair:      int16(binary.LittleEndian.Uint16(data[54:56])),
		Weapon:    int16(binary.LittleEndian.Uint16(data[56:58])),
		Level:     int16(binary.LittleEndian.Uint16(data[58:60])),
		HeadLow:   int16(binary.LittleEndian.Uint16(data[62:64])),
		Shield:    int16(binary.LittleEndian.Uint16(data[64:66])),
		HeadTop:   int16(binary.LittleEndian.Uint16(data[66:68])),
		HeadMid:   int16(binary.LittleEndian.Uint16(data[68:70])),
		Name:      fixedString(data[74:98]),
		Str:       data[98],
		Agi:       data[99],
		Vit:       data[100],
		Int:       data[101],
		Dex:       data[102],
		Luk:       data[103],
		Slot:      data[104],
		HairColor: data[105],
		HeadPal:   int16(binary.LittleEndian.Uint16(data[70:72])),
		BodyPal:   int16(binary.LittleEndian.Uint16(data[72:74])),
	}
}

func parseLegacyCharacter106(data []byte) Character {
	character := Character{
		ID:       binary.LittleEndian.Uint32(data[0:4]),
		Exp:      int64(int32(binary.LittleEndian.Uint32(data[4:8]))),
		Money:    int64(int32(binary.LittleEndian.Uint32(data[8:12]))),
		JobLevel: int16(binary.LittleEndian.Uint32(data[16:20])),
		HP:       int16(binary.LittleEndian.Uint16(data[42:44])),
		MaxHP:    int16(binary.LittleEndian.Uint16(data[44:46])),
		SP:       int16(binary.LittleEndian.Uint16(data[46:48])),
		MaxSP:    int16(binary.LittleEndian.Uint16(data[48:50])),
		Option:   binary.LittleEndian.Uint32(data[28:32]),
		Job:      int16(binary.LittleEndian.Uint16(data[52:54])),
		Hair:     int16(binary.LittleEndian.Uint16(data[54:56])),
		Weapon:   int16(binary.LittleEndian.Uint16(data[56:58])),
		Level:    int16(binary.LittleEndian.Uint16(data[58:60])),
		HeadLow:  int16(binary.LittleEndian.Uint16(data[62:64])),
		Shield:   int16(binary.LittleEndian.Uint16(data[64:66])),
		HeadTop:  int16(binary.LittleEndian.Uint16(data[66:68])),
		HeadMid:  int16(binary.LittleEndian.Uint16(data[68:70])),
		Name:     fixedString(data[74:98]),
		Str:      data[98],
		Agi:      data[99],
		Vit:      data[100],
		Int:      data[101],
		Dex:      data[102],
		Luk:      data[103],
	}
	if len(data) >= 106 {
		character.Slot = uint8(binary.LittleEndian.Uint16(data[104:106]))
	}
	character.HeadPal = int16(binary.LittleEndian.Uint16(data[70:72]))
	character.BodyPal = int16(binary.LittleEndian.Uint16(data[72:74]))
	if character.HairColor == 0 && character.HeadPal > 0 && character.HeadPal <= 255 {
		character.HairColor = uint8(character.HeadPal)
	}
	return character
}

func fixedString(data []byte) string {
	if index := strings.IndexByte(string(data), 0); index >= 0 {
		data = data[:index]
	}
	return strings.TrimSpace(string(data))
}
