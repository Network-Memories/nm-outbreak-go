package stun

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"
)

const (
	bindingRequest  = 0x0001
	bindingResponse = 0x0101
	magicCookie     = 0x2112A442
)

// Status represents the live state of the immutable NM STUN link.
type Status struct {
	Healthy               bool      `json:"healthy"`
	Addr                  string    `json:"addr"`
	LastCheck             time.Time `json:"last_check"`
	LastSuccess           time.Time `json:"last_success,omitempty"`
	LastError             string    `json:"last_error,omitempty"`
	RelayEnabled          bool      `json:"relay_enabled"`
	RelayState            string    `json:"relay_state"`
	GameTransportVerified bool      `json:"game_transport_verified"`
	MappedAddress         string    `json:"mapped_address,omitempty"`
}

// Link maintains the mandatory NM STUN link with periodic verification,
// explicit degraded state, and automatic reconnection. Active games are
// never interrupted by a temporary STUN outage.
type Link struct {
	addr     string
	interval time.Duration
	timeout  time.Duration
	healthy  atomic.Bool
	status   atomic.Value // Status
}

func NewLink(addr string, interval, timeout time.Duration) *Link {
	l := &Link{
		addr:     addr,
		interval: interval,
		timeout:  timeout,
	}
	l.status.Store(Status{Addr: addr, RelayEnabled: false, RelayState: "not_implemented"})
	return l
}

func (l *Link) Probe() error {
	conn, err := net.DialTimeout("udp", l.addr, l.timeout)
	if err != nil {
		l.record(false, err, "")
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(l.timeout))

	var req [20]byte
	binary.BigEndian.PutUint16(req[0:2], bindingRequest)
	binary.BigEndian.PutUint16(req[2:4], 0)
	binary.BigEndian.PutUint32(req[4:8], magicCookie)
	if _, err := rand.Read(req[8:20]); err != nil {
		l.record(false, err, "")
		return err
	}

	if _, err := conn.Write(req[:]); err != nil {
		l.record(false, err, "")
		return err
	}

	var resp [512]byte
	n, err := conn.Read(resp[:])
	if err != nil {
		l.record(false, err, "")
		return err
	}
	mapped, err := parseBindingResponse(resp[:n], req[8:20])
	if err != nil {
		l.record(false, err, "")
		return err
	}
	l.record(true, nil, mapped)
	return nil
}

func parseBindingResponse(raw, transaction []byte) (string, error) {
	if len(raw) < 20 || len(transaction) != 12 || binary.BigEndian.Uint16(raw[:2]) != bindingResponse || binary.BigEndian.Uint32(raw[4:8]) != magicCookie || string(raw[8:20]) != string(transaction) {
		return "", errors.New("invalid STUN binding response")
	}
	size := int(binary.BigEndian.Uint16(raw[2:4]))
	if size != len(raw)-20 || size%4 != 0 {
		return "", errors.New("invalid STUN response length")
	}
	mapped := ""
	for at := 20; at < len(raw); {
		if at+4 > len(raw) {
			return "", errors.New("truncated STUN attribute")
		}
		kind := binary.BigEndian.Uint16(raw[at:])
		length := int(binary.BigEndian.Uint16(raw[at+2:]))
		start := at + 4
		next := start + ((length + 3) &^ 3)
		if start+length > len(raw) || next > len(raw) {
			return "", errors.New("invalid STUN attribute length")
		}
		if kind == 0x0020 {
			value := raw[start : start+length]
			if length < 4 || value[0] != 0 {
				return "", errors.New("invalid XOR mapped address")
			}
			port := binary.BigEndian.Uint16(value[2:4]) ^ uint16(magicCookie>>16)
			var ip net.IP
			switch value[1] {
			case 1:
				if length != 8 {
					return "", errors.New("invalid IPv4 mapping")
				}
				ip = make(net.IP, 4)
				for i := 0; i < 4; i++ {
					ip[i] = value[4+i] ^ raw[4+i]
				}
			case 2:
				if length != 20 {
					return "", errors.New("invalid IPv6 mapping")
				}
				ip = make(net.IP, 16)
				for i := 0; i < 16; i++ {
					ip[i] = value[4+i] ^ raw[4+i]
				}
			default:
				return "", errors.New("unknown mapped address family")
			}
			if port == 0 || ip.IsUnspecified() {
				return "", errors.New("empty STUN mapping")
			}
			if mapped != "" {
				return "", errors.New("duplicate STUN mapping")
			}
			mapped = net.JoinHostPort(ip.String(), fmt.Sprint(port))
		}
		at = next
	}
	if mapped == "" {
		return "", errors.New("STUN response has no XOR mapped address")
	}
	return mapped, nil
}

func (l *Link) record(ok bool, err error, mapped string) {
	prev := l.Status()
	now := time.Now().UTC()
	st := Status{
		Healthy:       ok,
		Addr:          l.addr,
		LastCheck:     now,
		LastSuccess:   prev.LastSuccess,
		RelayEnabled:  false,
		RelayState:    "not_implemented",
		MappedAddress: prev.MappedAddress,
	}
	if ok {
		st.LastSuccess = now
		st.MappedAddress = mapped
		if !l.healthy.Swap(true) {
			log.Printf("[nm.stun] STUN probe responding; game transport not yet verified (%s)", l.addr)
		}
	} else {
		if err != nil {
			st.LastError = err.Error()
		}
		wasHealthy := l.healthy.Swap(false)
		if wasHealthy || prev.LastCheck.IsZero() {
			log.Printf("[nm.stun] NM STUN native link degraded (%s): %v (game relay not implemented)", l.addr, err)
		}
	}
	l.status.Store(st)
}

func (l *Link) Status() Status {
	return l.status.Load().(Status)
}

func (l *Link) Start(ctx context.Context) {
	_ = l.Probe()
	go func() {
		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = l.Probe()
			}
		}
	}()
}
