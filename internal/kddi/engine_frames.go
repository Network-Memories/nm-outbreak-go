package kddi

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/protocol/enginewire"
	"sync"
	"time"
)

type gameWriter struct {
	conn net.Conn
	mu   sync.Mutex
}

func (w *gameWriter) write(p []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	for len(p) > 0 {
		n, err := w.conn.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}
func (s *Server) relayMixedGame(c net.Conn, id string, gs *GameStream) {
	// Both actual native entry transitions are required before the server-side
	// Japanese adapter announces its Western transport endpoint.
	deadline := time.Now().Add(10 * time.Second)
	var ctx core.EngineContext
	for {
		var err error
		ctx, err = s.engine.MixedEngineContext(id)
		if err != nil {
			return
		}
		entered := true
		for _, p := range ctx.Peers {
			entered = entered && p.InScenario
		}
		if entered {
			break
		}
		if time.Now().After(deadline) {
			log.Printf("[nm.kddi.%s] peer native entry not observed", s.name)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	attachment, err := s.engine.AttachEngine(id)
	if err != nil {
		return
	}
	defer s.engine.DetachEngine(attachment)
	// Western seats are simulated server-side; actual Japanese seats exchange
	// their own records verbatim (measured reference server behaviour).
	var remotes []int
	japanesePeers := false
	for i, p := range ctx.Peers {
		switch {
		case p.ID == id:
		case p.Protocol == core.SNAP:
			remotes = append(remotes, i)
		case p.Protocol == core.KDDI:
			japanesePeers = true
		}
	}
	adapter, err := enginewire.NewKDDIPeers(ctx.Index, remotes)
	if err != nil {
		return
	}
	writer := &gameWriter{conn: c}
	for _, p := range adapter.Start() {
		if writer.write(p) != nil {
			return
		}
	}
	done, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	defer func() { cancel(); c.Close(); <-finished }()
	go func() {
		defer close(finished)
		defer c.Close()
		for {
			select {
			case <-done.Done():
				return
			case delivery, ok := <-attachment.Events():
				if !ok || !s.engine.EngineAttachmentActive(attachment) {
					return
				}
				p := delivery.Raw
				if p == nil {
					var err error
					p, err = enginewire.JapaneseEncode(delivery.Frame)
					if err != nil {
						return
					}
				}
				if writer.write(p) != nil {
					return
				}
			}
		}
	}()
	started := time.Now()
	var split enginewire.Reassembler
	buf := make([]byte, 4096)
	for {
		n, readErr := c.Read(buf)
		if n > 0 {
			records, err := gs.Feed(buf[:n])
			if err != nil {
				return
			}
			s.engine.MarkAlive(id)
			for _, raw := range records {
				if !s.engine.EngineAttachmentActive(attachment) {
					return
				}
				if japanesePeers {
					until := time.Now().Add(5 * time.Second)
					for {
						err = s.engine.PublishKDDIRaw(id, ctx.RoundID, raw)
						if err == nil {
							break
						}
						if !errors.Is(err, core.ErrEngineBackpressure) || time.Now().After(until) || !s.engine.EngineAttachmentActive(attachment) {
							log.Printf("[nm.kddi.%s] native record relay refused: %v", s.name, err)
							return
						}
						time.Sleep(20 * time.Millisecond)
					}
				}
				record, splitErr := split.Feed(raw)
				if splitErr != nil {
					log.Printf("[nm.kddi.%s] split engine record refused: % x", s.name, raw)
					return
				}
				if record == nil {
					continue
				}
				replies, frame, err := adapter.Receive(record, float32(time.Since(started).Seconds()))
				if err != nil {
					head := record
					if len(head) > 16 {
						head = head[:16]
					}
					log.Printf("[nm.kddi.%s] mixed game transport record refused after %.1fs: %v (len=%d head=% x)", s.name, time.Since(started).Seconds(), err, len(record), head)
					return
				}
				for _, p := range replies {
					if writer.write(p) != nil {
						return
					}
				}
				if frame != nil {
					until := time.Now().Add(5 * time.Second)
					for {
						err = s.engine.PublishEngine(id, ctx.RoundID, []enginewire.Frame{*frame})
						if err == nil {
							break
						}
						if !errors.Is(err, core.ErrEngineBackpressure) || time.Now().After(until) || !s.engine.EngineAttachmentActive(attachment) {
							log.Printf("[nm.kddi.%s] mixed engine input refused: %v", s.name, err)
							return
						}
						time.Sleep(20 * time.Millisecond)
					}
				}
			}
		}
		if readErr != nil {
			return
		}
	}
}
