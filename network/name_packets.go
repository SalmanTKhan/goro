package network

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const nameLength = 24

type ActorNameAck struct {
	ID           uint32
	Name         string
	PartyName    string
	GuildName    string
	Title        string
	HasGuildName bool
}

func ParseActorNameAck(packet Packet) (ActorNameAck, bool, error) {
	switch packet.ID {
	case 0x0095, 0x0194:
		end := 6 + nameLength
		if len(packet.Data) < end {
			return ActorNameAck{}, false, fmt.Errorf("name ack 0x%04X too short: %d", packet.ID, len(packet.Data))
		}
		return ActorNameAck{
			ID:   binary.LittleEndian.Uint32(packet.Data[2:6]),
			Name: fixedPacketString(packet.Data[6:end]),
		}, true, nil
	case 0x0195:
		end := 6 + nameLength*4
		if len(packet.Data) < end {
			return ActorNameAck{}, false, fmt.Errorf("name all ack 0x%04X too short: %d", packet.ID, len(packet.Data))
		}
		return ActorNameAck{
			ID:           binary.LittleEndian.Uint32(packet.Data[2:6]),
			Name:         fixedPacketString(packet.Data[6 : 6+nameLength]),
			PartyName:    fixedPacketString(packet.Data[30 : 30+nameLength]),
			GuildName:    fixedPacketString(packet.Data[54 : 54+nameLength]),
			Title:        fixedPacketString(packet.Data[78:end]),
			HasGuildName: true,
		}, true, nil
	case 0x0A30:
		end := 6 + nameLength*4
		if len(packet.Data) < end+4 {
			return ActorNameAck{}, false, fmt.Errorf("name all2 ack 0x%04X too short: %d", packet.ID, len(packet.Data))
		}
		return ActorNameAck{
			ID:           binary.LittleEndian.Uint32(packet.Data[2:6]),
			Name:         fixedPacketString(packet.Data[6 : 6+nameLength]),
			PartyName:    fixedPacketString(packet.Data[30 : 30+nameLength]),
			GuildName:    fixedPacketString(packet.Data[54 : 54+nameLength]),
			Title:        fixedPacketString(packet.Data[78:end]),
			HasGuildName: true,
		}, true, nil
	case 0x0ADF:
		end := 10 + nameLength*2
		if len(packet.Data) < end {
			return ActorNameAck{}, false, fmt.Errorf("name all3 ack 0x%04X too short: %d", packet.ID, len(packet.Data))
		}
		return ActorNameAck{
			ID:           binary.LittleEndian.Uint32(packet.Data[2:6]),
			Name:         fixedPacketString(packet.Data[10 : 10+nameLength]),
			GuildName:    fixedPacketString(packet.Data[34:end]),
			HasGuildName: true,
		}, true, nil
	case 0x0AF7:
		end := 8 + nameLength
		if len(packet.Data) < end {
			return ActorNameAck{}, false, fmt.Errorf("name bygid2 ack 0x%04X too short: %d", packet.ID, len(packet.Data))
		}
		return ActorNameAck{
			ID:   binary.LittleEndian.Uint32(packet.Data[4:8]),
			Name: fixedPacketString(packet.Data[8:end]),
		}, true, nil
	default:
		return ActorNameAck{}, false, nil
	}
}

func fixedPacketString(data []byte) string {
	if nul := strings.IndexByte(string(data), 0); nul >= 0 {
		data = data[:nul]
	}
	return strings.TrimSpace(string(data))
}
