package kddi

import (
	"bytes"
	"net"
	"testing"
	"time"
)

type shortLobbyConn struct{ net.Conn }

func (c shortLobbyConn) Write(p []byte) (int, error) {
	if len(p) > 3 {
		p = p[:3]
	}
	return c.Conn.Write(p)
}
func TestConcurrentBroadcastAndReplyNeverInterleavePartialFrames(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	writer := &lobbyFrameConn{Conn: shortLobbyConn{server}}
	expected := map[uint16][]byte{0x6503: bytes.Repeat([]byte{0x33}, 60), 0x6911: bytes.Repeat([]byte{0x11}, 40)}
	errors := make(chan error, 2)
	start := make(chan struct{})
	for cmd, data := range expected {
		go func(cmd uint16, data []byte) {
			<-start
			errors <- writeFrame(writer, LobbyFrame{Sender: 0x18, Kind: 0x10, Command: cmd, PacketID: 8, Payload: data})
		}(cmd, data)
	}
	close(start)
	for i := 0; i < 2; i++ {
		f, err := readFrame(client)
		if err != nil || !bytes.Equal(f.Payload, expected[f.Command]) {
			t.Fatal("native frames interleaved", f, err)
		}
		delete(expected, f.Command)
	}
	for i := 0; i < 2; i++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
}
