package network

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestClientCloseDoesNotReportReadLoopError(t *testing.T) {
	client := NewClient(20080910, false)
	local, remote := net.Pipe()
	defer remote.Close()

	client.mu.Lock()
	client.conn = local
	client.mu.Unlock()
	done := make(chan struct{})
	go func() {
		client.readLoop(local)
		close(done)
	}()

	client.Close()
	select {
	case <-done:
		if errs := client.DrainErrors(); len(errs) > 0 {
			t.Fatalf("intentional close produced errors: %v", errs)
		}
	case <-time.After(time.Second):
		t.Fatal("read loop did not exit")
	}
}

func TestClientReadLoopReportsRemoteDisconnect(t *testing.T) {
	client := NewClient(20080910, false)
	local, remote := net.Pipe()

	client.mu.Lock()
	client.conn = local
	client.mu.Unlock()
	done := make(chan struct{})
	go func() {
		client.readLoop(local)
		close(done)
	}()

	_ = remote.Close()
	select {
	case <-done:
		errs := client.DrainErrors()
		if len(errs) != 1 {
			t.Fatalf("errors = %v, want one disconnect error", errs)
		}
		if !errors.Is(errs[0], ErrDisconnected) {
			t.Fatalf("error = %v, want ErrDisconnected", errs[0])
		}
	case <-time.After(time.Second):
		t.Fatal("read loop did not exit")
	}
}

func TestClientSendQueuesWithoutBlockingOnSocketWrite(t *testing.T) {
	client := NewClient(20080910, false)
	local, remote := net.Pipe()
	defer remote.Close()

	sendCh := make(chan outboundPacket, sendQueueSize)
	client.mu.Lock()
	client.conn = local
	client.sendCh = sendCh
	client.mu.Unlock()
	go client.writeLoop(local, sendCh)

	done := make(chan error, 1)
	go func() {
		done <- client.Send([]byte{0x01, 0x02, 0x03})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Send returned error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Send blocked on socket write")
	}

	client.Close()
}

func TestClientSendPingQueuesCharKeepalive(t *testing.T) {
	client := NewClient(20080910, false)
	local, remote := net.Pipe()
	defer remote.Close()

	sendCh := make(chan outboundPacket, sendQueueSize)
	client.mu.Lock()
	client.conn = local
	client.sendCh = sendCh
	client.mu.Unlock()
	go client.writeLoop(local, sendCh)

	if err := client.SendPing(0x11223344); err != nil {
		t.Fatalf("SendPing returned error: %v", err)
	}

	if err := remote.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 6)
	if _, err := io.ReadFull(remote, packet); err != nil {
		t.Fatalf("reading ping packet: %v", err)
	}
	if binary.LittleEndian.Uint16(packet[0:2]) != PacketPing {
		t.Fatalf("opcode = % x", packet[:2])
	}
	if binary.LittleEndian.Uint32(packet[2:6]) != 0x11223344 {
		t.Fatalf("account id = 0x%08x", binary.LittleEndian.Uint32(packet[2:6]))
	}

	client.Close()
}

func TestClientSendQuitGameAndCloseFlushesBeforeClose(t *testing.T) {
	client := NewClient(20080910, false)
	local, remote := net.Pipe()
	defer remote.Close()

	sendCh := make(chan outboundPacket, sendQueueSize)
	client.mu.Lock()
	client.conn = local
	client.sendCh = sendCh
	client.mu.Unlock()
	go client.writeLoop(local, sendCh)

	done := make(chan error, 1)
	go func() { done <- client.SendQuitGameAndClose() }()

	if err := remote.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 4)
	if _, err := io.ReadFull(remote, packet); err != nil {
		t.Fatalf("reading disconnect packet: %v", err)
	}
	if binary.LittleEndian.Uint16(packet) != PacketCZQuitGame {
		t.Fatalf("opcode = 0x%04x, want 0x%04x", binary.LittleEndian.Uint16(packet), PacketCZQuitGame)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SendQuitGameAndClose returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SendQuitGameAndClose did not complete")
	}
}
