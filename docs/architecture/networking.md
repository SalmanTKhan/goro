# Networking (`network`)

Encodes/decodes Ragnarok Online packets and owns socket I/O. It exposes parsed
protocol facts; it makes no UI or gameplay decisions — reactions live in `game`.

## Client

`network.NewClient(clientDate, trace)`. `Connect(ctx, address, port)` dials with
a 5s timeout and starts a read loop and a write loop goroutine (send queue of
256 packets). Received packets accumulate for the game loop to drain; errors
accumulate too, with `ErrDisconnected` and `FrameError` as the notable types.
`status` is a human-readable connection string surfaced in the UI.

Three servers are contacted in sequence: login (account) → char → zone (map),
each a fresh `Connect`.

## Framing

`framer.go` splits the TCP stream using a `LengthTable` (`PacketLengths2008()`
in `packet.go`) mapping packet ID to a fixed length, or `-1` for
length-prefixed variable packets. A `Packet` is `{ID uint16; Data []byte}`.

Client date drives the packet profile (`[packet] client_date`, `profile`);
`20080910` is the primary target. `--net-trace` traces traffic.

## Layout

One file per feature area, each with a `_test.go` beside it:

`login_packets.go` / `login_responses.go`, `map_packets.go`, `actor_packets.go`,
`chat_packets.go`, `item_packets.go`, `equipment_packets.go`, `skill_packets.go`,
`status_packets.go`, `npc_packets.go`, `party_packets.go`, `guild_packets.go`,
`friend_packets.go`, `pet_packets.go`, `companion_packets.go`,
`trade_packets.go`, `vending_packets.go`, `emotion_packets.go`,
`effect_packets.go`, `minimap_packets.go`, `name_packets.go`, `hotkey_packets.go`,
`pvp_packets.go`, `restart_packets.go`, `disconnect_packets.go`,
`adoption_packets.go`, `taekwon_packets.go`.

Helpers: `writer.go` (outgoing packet builder), `fixed_string.go` (fixed-width
string fields).

## Adding a packet

1. Add its ID and length to `PacketLengths2008()`.
2. Add a parser or builder in the matching `*_packets.go`, returning a plain
   struct with no side effects.
3. Add a table-driven test with real byte fixtures in the sibling `_test.go`.
4. React to it in `game` (`world_packets.go`, `session_state_updates.go`, or the
   feature file) — never inside `network`.

Coverage against the reference client is tracked in
`docs/packet-coverage-20080910.md` and
`docs/packet-coverage-20080910-per-feature.md`.
