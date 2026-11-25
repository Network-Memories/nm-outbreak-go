package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/protocol/enginewire"
	"networkmemories/nm-outbreak-go/internal/protocol/snaplaunch"
	"testing"
)

func enteredMixed(t *testing.T) (*Engine, Session, Session) {
	e, h, g, _, _, _ := launchActors(t)
	if err := e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 3}); err != nil {
		t.Fatal(err)
	}
	e.AcknowledgeNativeLaunch(g.ID)
	feedLaunch(t, e, h, g)
	if err := e.ScenarioPresence(h.ID, true); err != nil {
		t.Fatal(err)
	}
	return e, h, g
}
func TestEngineHandoffQueuesOriginalBodiesAndBindsSender(t *testing.T) {
	e, h, g := enteredMixed(t)
	ctx, err := e.MixedEngineContext(h.ID)
	if err != nil || ctx.Index != 0 {
		t.Fatal(ctx, err)
	}
	if _, err = e.AttachEngine(g.ID); err == nil {
		t.Fatal("unentered native guest attached")
	}
	body := []byte{1, 2, 3}
	if err = e.PublishEngine(h.ID, ctx.RoundID, []enginewire.Frame{{Peer: 0, Body: body}}); err != nil {
		t.Fatal(err)
	}
	body[0] = 99
	e.ScenarioPresence(g.ID, true)
	receiver, err := e.AttachEngine(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.AttachEngine(g.ID); !errors.Is(err, ErrEngineAttached) {
		t.Fatal("two transports consumed one place", err)
	}
	packet := <-receiver.Events()
	if packet.Frame.Peer != 0 || packet.Frame.Body[0] != 1 || packet.FromID != h.ID {
		t.Fatal(packet)
	}
	if e.PublishEngine(g.ID, ctx.RoundID, []enginewire.Frame{{Peer: 0, Body: []byte{7}}}) == nil {
		t.Fatal("native guest forged host place")
	}
	if e.PublishEngine(h.ID, "old-round", []enginewire.Frame{{Peer: 0}}) == nil {
		t.Fatal("stale transport crossed rounds")
	}
	e.DetachEngine(receiver)
	again, err := e.AttachEngine(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	e.DetachEngine(receiver)
	if !e.EngineAttachmentActive(again) {
		t.Fatal("old detach stole replacement socket")
	}
	e.LeaveRoom(g.ID)
	if e.EngineAttachmentActive(again) {
		t.Fatal("departed peer active")
	}
	if _, ok := <-again.Events(); ok {
		t.Fatal("departed round mailbox not closed")
	}
	if h2, ok := e.Session(h.ID); !ok || !h2.InScenario {
		t.Fatal("socket cleanup invented host return")
	}
}
func TestEngineBatchBackpressureIsAtomicAndBounded(t *testing.T) {
	e, h, g := enteredMixed(t)
	ctx, _ := e.MixedEngineContext(h.ID)
	for i := 0; i < engineQueueSize-1; i++ {
		if err := e.PublishEngine(h.ID, ctx.RoundID, []enginewire.Frame{{Peer: 0, Body: []byte{byte(i)}}}); err != nil {
			t.Fatal(err)
		}
	}
	err := e.PublishEngine(h.ID, ctx.RoundID, []enginewire.Frame{{Peer: 0}, {Peer: 0}})
	if !errors.Is(err, ErrEngineBackpressure) {
		t.Fatal(err)
	}
	e.ScenarioPresence(g.ID, true)
	a, _ := e.AttachEngine(g.ID)
	if len(a.mailbox.events) != engineQueueSize-1 {
		t.Fatal("rejected batch partially committed")
	}
	<-a.Events()
	if err = e.PublishEngine(h.ID, ctx.RoundID, []enginewire.Frame{{Peer: 0}, {Peer: 0}}); err != nil {
		t.Fatal("drain did not recover", err)
	}
	if len(a.mailbox.events) != engineQueueSize {
		t.Fatal("frame lost after retry")
	}
}
