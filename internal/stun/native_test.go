package stun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestNativeControlDatapathAndControlOutage(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "nm-native-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	control := filepath.Join(dir, "control.sock")
	channel := filepath.Join(dir, "channel.sock")
	ln, err := net.Listen("unix", control)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	udp, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: channel, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	requests := make(chan map[string]any, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			var req map[string]any
			_ = json.NewDecoder(bufio.NewReader(c)).Decode(&req)
			requests <- req
			_ = json.NewEncoder(c).Encode(map[string]any{"ok": true, "canal": channel})
			c.Close()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	native, err := ListenNative(ctx, control, "0.0.0.0", 9091)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	req := <-requests
	if !regexp.MustCompile(`^client-[0-9a-f]{24}\.sock$`).MatchString(filepath.Base(req["client"].(string))) {
		t.Fatal("native control socket name violates existing contract")
	}
	if req["operation"] != "reserver" || req["port"] != float64(9091) || len(req["proprietaire"].(string)) != 32 {
		t.Fatal("shared control contract mismatch")
	}
	if st, err := os.Stat(req["client"].(string)); err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("native client endpoint is not private")
	}
	peer := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 23), Port: 4567}
	payload := []byte("original-native-frame")
	if n, err := native.WriteTo(payload, peer); err != nil || n != len(payload) {
		t.Fatal("native send", err)
	}
	udp.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 2048)
	n, from, err := udp.ReadFromUnix(buf)
	if err != nil || !bytes.Equal(buf[6:n], payload) || binary.BigEndian.Uint16(buf[4:6]) != 4567 {
		t.Fatal("game did not use shared transport", err)
	}
	if _, err = udp.WriteToUnix(buf[:n], from); err != nil {
		t.Fatal(err)
	}
	native.SetReadDeadline(time.Now().Add(time.Second))
	got := make([]byte, 2048)
	size, addr, err := native.ReadFrom(got)
	if err != nil || addr.String() != peer.String() || !bytes.Equal(got[:size], payload) {
		t.Fatal("native receive", err)
	}
	ln.Close()
	if native.reserve() == nil || native.Status().Connected {
		t.Fatal("control outage hidden")
	}
	// Control-plane failure leaves the existing native data channel usable.
	if _, err = native.WriteTo(payload, peer); err != nil {
		t.Fatal("control outage interrupted established channel", err)
	}
	n, from, err = udp.ReadFromUnix(buf)
	if err != nil {
		t.Fatal(err)
	}
	udp.WriteToUnix(buf[:n], from)
	if _, _, err = native.ReadFrom(got); err != nil {
		t.Fatal(err)
	}
	if native.Status().Sent != 2 || native.Status().Received != 2 {
		t.Fatal("transport counters missing")
	}
	// A full restart also destroys the data channel. Keep the same Go
	// transport and verify that its periodic reservation attaches the new one.
	if err := udp.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := native.WriteTo(payload, peer); err == nil {
		t.Fatal("lost data channel reported a successful send")
	}
	restartedChannel := filepath.Join(dir, "restarted.sock")
	restartedUDP, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: restartedChannel, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer restartedUDP.Close()
	restartedControl, err := net.ListenUnix("unix", &net.UnixAddr{Name: control, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer restartedControl.Close()
	restartedControl.SetDeadline(time.Now().Add(5 * time.Second))
	c, err := restartedControl.AcceptUnix()
	if err != nil {
		t.Fatal("periodic reservation did not reconnect", err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	var renewal map[string]any
	if err := json.NewDecoder(c).Decode(&renewal); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"ip", "port", "proprietaire", "client"} {
		if renewal[field] != req[field] {
			t.Fatalf("reservation changed %s after controller restart", field)
		}
	}
	if err := json.NewEncoder(c).Encode(map[string]any{"ok": true, "canal": restartedChannel}); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(time.Second); !native.Status().Connected; {
		if time.Now().After(deadline) {
			t.Fatal("controller recovery did not update transport status")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := native.WriteTo(payload, peer); err != nil {
		t.Fatal("data channel did not recover", err)
	}
	restartedUDP.SetReadDeadline(time.Now().Add(time.Second))
	n, from, err = restartedUDP.ReadFromUnix(buf)
	if err != nil || n != len(payload)+6 || !bytes.Equal(buf[6:n], payload) {
		t.Fatal("payload missing from replacement channel", err)
	}
	if _, err := restartedUDP.WriteToUnix(buf[:n], from); err != nil {
		t.Fatal(err)
	}
	native.SetReadDeadline(time.Now().Add(time.Second))
	size, addr, err = native.ReadFrom(got)
	if err != nil || addr.String() != peer.String() || !bytes.Equal(got[:size], payload) {
		t.Fatal("replacement channel did not return payload", err)
	}
	native.Close()
	if _, err = os.Stat(req["client"].(string)); !os.IsNotExist(err) {
		t.Fatal("client socket left behind")
	}
}
func TestNativeControlFailureHasNoDirectFallback(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "nm-native-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if _, err = ListenNative(context.Background(), filepath.Join(dir, "absent.sock"), "127.0.0.1", 9092); err == nil {
		t.Fatal("missing control silently bypassed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed reservation leaked socket")
	}
}
