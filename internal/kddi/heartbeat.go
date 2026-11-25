// KDDI lobby liveness follows BioServer HeartBeatThread/PacketHandler.java.
package kddi

import (
	"context"
	"net"
	"sync/atomic"
	"time"
)

type nativeHeartbeat struct{ awaiting atomic.Bool }

// Payload is opaque module data. It is not a population or a game statistic.
func (h *nativeHeartbeat) run(ctx context.Context, c net.Conn, nextID func() uint16, payload []byte, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(payload) > 0 {
				if writeFrame(c, LobbyFrame{Sender: SenderServer, Kind: 0x10, Command: 0x6202, PacketID: nextID(), Payload: payload}) != nil {
					c.Close()
					return
				}
			}
			ticks++
			if ticks%2 == 0 {
				if !h.awaiting.CompareAndSwap(false, true) {
					c.Close()
					return
				}
				if writeFrame(c, LobbyFrame{Sender: SenderServer, Kind: 1, Command: CmdKeepAlive, PacketID: nextID()}) != nil {
					c.Close()
					return
				}
			}
		}
	}
}
