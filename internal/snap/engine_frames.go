package snap

import (
	"errors"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/protocol/enginewire"
	"time"
)

func (s *Server) watchEngine(pc net.PacketConn, id string) {
	ctx, err := s.engine.MixedEngineContext(id)
	if err != nil {
		return
	}
	s.notifyMu.Lock()
	if s.engineWatchers == nil {
		s.engineWatchers = make(map[string]string)
	}
	if s.engineWatchers[id] == ctx.RoundID {
		s.notifyMu.Unlock()
		return
	}
	attachment, err := s.engine.AttachEngine(id)
	if err != nil {
		s.notifyMu.Unlock()
		return
	}
	s.engineWatchers[id] = ctx.RoundID
	s.notifyMu.Unlock()
	go func() {
		defer func() {
			s.engine.DetachEngine(attachment)
			s.notifyMu.Lock()
			if s.engineWatchers[id] == ctx.RoundID {
				delete(s.engineWatchers, id)
			}
			s.notifyMu.Unlock()
		}()
		for {
			select {
			case <-s.notifyDone:
				return
			case delivery, ok := <-attachment.Events():
				if !ok {
					return
				}
				if !s.engine.EngineAttachmentActive(attachment) {
					return
				}
				b, ok := s.bindings.FindCore(id)
				if !ok || b.Addr == nil {
					return
				}
				p, err := enginewire.WesternEncode(delivery.Frame)
				if err != nil {
					log.Printf("[nm.snap.%s] mixed engine envelope refused", s.name)
					return
				}
				flags := uint16(FlagRoom)
				if len(delivery.Frame.Body) > 0 {
					flags |= FlagReliable
				}
				until := time.Now().Add(5 * time.Second)
				for {
					err = s.sendReply(pc, b.Addr, Message{TypeFlags: flags, Command: CmdSend, SessionID: delivery.WireID, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: p})
					if err == nil {
						break
					}
					if !errors.Is(err, ErrReliableBackpressure) || time.Now().After(until) || !s.engine.EngineAttachmentActive(attachment) {
						log.Printf("[nm.snap.%s] mixed engine output failed: %v", s.name, err)
						return
					}
					select {
					case <-s.notifyDone:
						return
					case <-time.After(20 * time.Millisecond):
					}
				}
			}
		}
	}()
}
func (s *Server) relayMixedEngine(sender core.Session, payload []byte) {
	ctx, err := s.engine.MixedEngineContext(sender.ID)
	if err != nil {
		return
	}
	frames, err := enginewire.WesternDecode(payload, ctx.Count)
	if err != nil {
		log.Printf("[nm.snap.%s] malformed mixed engine batch", s.name)
		return
	}
	real := frames[:0]
	for _, f := range frames {
		if f.Peer != 15 {
			real = append(real, f)
		}
	}
	if len(real) == 0 {
		return
	}
	if err = s.engine.PublishEngine(sender.ID, ctx.RoundID, real); err != nil {
		log.Printf("[nm.snap.%s] mixed engine input refused: %v", s.name, err)
	}
}

// Called by reliable ordering before advancing the accepted sequence. A full
// destination stays in the bounded transport queue and is retried by the tick.
func (s *Server) admitEngineGroup(remote net.Addr, ms []Message) bool {
	if len(ms) == 0 {
		return true
	}
	hasEngine := false
	for _, m := range ms {
		if m.Command == CmdSend && m.TypeFlags&FlagChannelBits == FlagRoom && m.TypeFlags&0x0400 == 0 {
			hasEngine = true
			break
		}
	}
	if !hasEngine {
		return true
	}
	binding, ok := s.bindings.Get(ms[0].SessionID)
	if !ok || binding.Addr == nil || remote == nil || binding.Addr.Network() != remote.Network() || binding.Addr.String() != remote.String() {
		return true
	}
	sender, ok := s.engine.Session(binding.CoreID)
	if !ok || !sender.InScenario {
		return true
	}
	ctx, err := s.engine.MixedEngineContext(sender.ID)
	if err != nil {
		return true
	}
	var frames []enginewire.Frame
	var indexes []int
	for i, m := range ms {
		if m.Command != CmdSend || m.TypeFlags&FlagChannelBits != FlagRoom || m.TypeFlags&0x0400 != 0 {
			continue
		}
		parsed, err := enginewire.WesternDecode(m.Payload, ctx.Count)
		if err != nil {
			ms[i].EngineHandled = true
			continue
		}
		valid := true
		for _, f := range parsed {
			if f.Peer != 15 && (f.Peer != ctx.Index || len(f.Body) > 251) {
				valid = false
			}
		}
		if !valid {
			ms[i].EngineHandled = true
			continue
		}
		for _, f := range parsed {
			if f.Peer != 15 {
				frames = append(frames, f)
			}
		}
		indexes = append(indexes, i)
	}
	if len(frames) > 0 {
		err = s.engine.PublishEngine(sender.ID, ctx.RoundID, frames)
		if errors.Is(err, core.ErrEngineBackpressure) {
			return false
		}
		if err != nil {
			for _, i := range indexes {
				ms[i].EngineHandled = true
			}
			return true
		}
	}
	for _, i := range indexes {
		ms[i].EngineHandled = true
	}
	return true
}
