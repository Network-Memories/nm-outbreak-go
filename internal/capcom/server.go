// Capcom APP session and request flows ported from openSNAP capcom/server.py
// and outbreak{,2}/service.py (AGPL-3.0). No executable patch offer is served.
package capcom

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/reports"
	"networkmemories/nm-outbreak-go/internal/world"
)

// Compatibility aliases: both codecs use the common private report journal.
type Report = reports.Envelope
type ReportSink = reports.Sink
type FileSink = reports.FileSink
type Population func(world.Game, string) uint16
type Server struct {
	addr, host string
	titles     map[byte]modules.AppDefinition
	sink       ReportSink
	population Population
	limit      chan struct{}
}

func NewServer(addr, host string, sink ReportSink, population Population, defs ...modules.AppDefinition) (*Server, error) {
	if net.ParseIP(host) == nil || net.ParseIP(host).To4() == nil || net.ParseIP(host).IsUnspecified() {
		return nil, errors.New("capcom: reachable advertised IPv4 required")
	}
	s := &Server{addr: addr, host: host, titles: make(map[byte]modules.AppDefinition), sink: sink, population: population, limit: make(chan struct{}, 64)}
	for _, d := range defs {
		for _, build := range d.Builds {
			if _, ok := s.titles[build]; ok {
				return nil, errors.New("capcom: duplicate build")
			}
			s.titles[build] = d.Clone()
		}
	}
	return s, nil
}
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			select {
			case s.limit <- struct{}{}:
				go func() { defer func() { <-s.limit }(); s.serve(ctx, c) }()
			default:
				c.Close()
			}
		}
	}()
	return nil
}
func send(c io.Writer, f Frame) error {
	b, err := f.Encode()
	if err != nil {
		return err
	}
	for len(b) > 0 {
		n, err := c.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func (s *Server) serve(ctx context.Context, c net.Conn) {
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	c.SetDeadline(time.Now().Add(30 * time.Second))
	if send(c, Frame{Direction: 1, Command: 0x1002, Tail: 0xff, Payload: []byte{0, 0}}) != nil {
		return
	}
	f, err := ReadFrame(c)
	if err != nil || f.Command != 0x1002 || f.Direction != 2 || f.Error != 0 {
		return
	}
	login, n, err := DecodeField(f.Payload, f.Sequence)
	if err != nil || n+2 > len(f.Payload) {
		return
	}
	d, ok := s.titles[f.Payload[n]]
	if !ok {
		return
	}
	version, used, err := DecodeField(f.Payload[n+2:], f.Sequence)
	if err != nil || n+2+used+4 != len(f.Payload) {
		return
	}
	if string(version) == "0312181725" {
		return
	} // Public beta is a distinct unsupported module.
	name := strings.SplitN(string(login), "\x00", 2)[0]
	if name == "" || len(name) > 32 {
		return
	}
	// Version push byte 0 explicitly means no patch, per native protocol.
	if send(c, Frame{Direction: 0x10, Command: 0x6001, Tail: 0xff, Payload: []byte{0}}) != nil {
		return
	}
	pages := make(map[string][]byte)
	for {
		c.SetDeadline(time.Now().Add(30 * time.Second))
		f, err = ReadFrame(c)
		if err != nil {
			return
		}
		if f.Direction != 1 || f.Error != 0 {
			return
		}
		if f.Command == 0x1004 {
			if send(c, Frame{Direction: 2, Command: f.Command, Sequence: f.Sequence, Tail: 0xff}) != nil {
				return
			}
			// Native clients must consume the final reply before peer FIN.
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			var discard [1024]byte
			for {
				if _, err := c.Read(discard[:]); err != nil {
					break
				}
			}
			return
		}
		host, _, _ := net.SplitHostPort(c.RemoteAddr().String())
		data, err := s.replyFrom(d, f, name, pages, host)
		code := byte(0)
		if err != nil {
			code = 0xff
		}
		if send(c, Frame{Direction: 2, Command: f.Command, Sequence: f.Sequence, Error: code, Tail: 0xff, Payload: data}) != nil {
			return
		}
		if err != nil {
			return
		}
	}
}
func be16(v uint16) []byte            { b := make([]byte, 2); binary.BigEndian.PutUint16(b, v); return b }
func be32(v uint32) []byte            { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }
func field(b []byte, seq byte) []byte { out, _ := EncodeField(b, seq); return out }
func (s *Server) reply(d modules.AppDefinition, f Frame, login string, pages map[string][]byte) ([]byte, error) {
	return s.replyFrom(d, f, login, pages, "")
}
func (s *Server) replyFrom(d modules.AppDefinition, f Frame, login string, pages map[string][]byte, host string) ([]byte, error) {
	p := f.Payload
	seq := f.Sequence
	switch f.Command {
	case 0x6101:
		if len(p) < 4 {
			return nil, errCodec
		}
		name, _, err := DecodeField(p[4:], seq)
		if err != nil {
			return nil, err
		}
		page := []byte{0}
		pages[string(name)] = page
		b := append([]byte{1, 0, 0, 0, 0}, field(name, seq)...)
		return append(b, be32(uint32(len(page)))...), nil
	case 0x6102:
		name, n, err := DecodeField(p, seq)
		if err != nil || len(p) != n+6 {
			return nil, errCodec
		}
		page, ok := pages[string(name)]
		if !ok {
			return nil, errCodec
		}
		off := int(binary.BigEndian.Uint32(p[n:]))
		size := int(binary.BigEndian.Uint16(p[n+4:]))
		if off >= len(page) {
			return nil, errCodec
		}
		end := off + size
		if end > len(page) {
			end = len(page)
		}
		b := append(field(name, seq), be32(uint32(off))...)
		return append(b, field(page[off:end], seq)...), nil
	case 0x6401:
		return []byte{0, 0}, nil
	case 0x6301:
		if s.sink == nil {
			return nil, errors.New("capcom: result archive unavailable")
		}
		report := Report{RemoteHost: host, Game: d.Game, Protocol: "snap", Command: 0x6301, ClaimedLogin: login, Sequence: seq, Payload: append([]byte(nil), p...), ReceivedAt: time.Now().UTC()}
		if decoded, err := DecodeScenarioResult(p, seq); err == nil {
			report.Normalized = &decoded.Stats
			report.PayloadLogin = decoded.Login
		}
		return nil, s.sink.Archive(report)
	}
	if d.AreaKey == "" { // File 1's measured data commands.
		switch f.Command {
		case 0x6201:
			return fileList(d, seq), nil
		case 0x6202:
			return fileChunk(d, f)
		}
		return nil, errCodec
	}
	switch f.Command {
	case 0x6110:
		b := append([]byte{0, 0}, field(nil, seq)...)
		return append(b, field([]byte(s.host), seq)...), nil
	case 0x6220:
		return []byte{1, 0, 0, 0, 0}, nil
	case 0x6210:
		return make([]byte, 68), nil
	case 0x6103, 0x6105, 0x6403:
		return []byte{0, 0}, nil
	case 0x1007:
		return []byte{0}, nil
	case 0x6211, 0x6212:
		return make([]byte, 4), nil
	case 0x6203:
		return fileList(d, seq), nil
	case 0x6204:
		return fileChunk(d, f)
	case 0x6213:
		var b []byte
		for _, v := range []uint16{20, 300, 300, 5400, 1800, 1800, 1800, 1800} {
			b = append(b, be16(v)...)
		}
		return b, nil
	case 0x6501:
		if len(p) != 4 {
			return nil, errCodec
		}
		if binary.BigEndian.Uint32(p) == 0 {
			return []byte{1, 0, 0, 0, 0, 0, 0, 0, 1}, nil
		}
		return []byte{0}, nil
	case 0x6502:
		return append([]byte{1}, make([]byte, 8)...), nil
	case 0x6503, 0x6504:
		if len(p) < 4 {
			return nil, errCodec
		}
		start := binary.BigEndian.Uint16(p)
		b := append(be16(1), be16(start)...)
		if start != 0 {
			return append(b, 0), nil
		}
		b = append(b, 1)
		key := s.host
		if f.Command == 0x6504 {
			key = d.AreaKey
		}
		b = append(b, field([]byte(key), seq)...)
		b = append(b, field([]byte("Outbreak"), seq)...)
		b = append(b, make([]byte, 8)...)
		b = append(b, field([]byte("Resident Evil Outbreak"), seq)...)
		if f.Command == 0x6504 {
			b = append(b, be16(d.AreaCount)...)
			b = append(b, be16(d.Capacity)...)
			b = append(b, make([]byte, 4)...)
		}
		return b, nil
	case 0x6510:
		if len(p) < 1 {
			return nil, errCodec
		}
		n := 1
		b := []byte{p[0]}
		for i := 0; i < int(p[0]); i++ {
			key, used, err := DecodeField(p[n:], seq)
			if err != nil {
				return nil, err
			}
			n += used
			pop := uint16(0)
			if s.population != nil {
				pop = s.population(d.Game, strings.TrimRight(string(key), "\x00"))
			}
			b = append(b, be16(pop)...)
		}
		_, used, err := DecodeField(p[n:], seq)
		if err != nil || n+used != len(p) {
			return nil, errCodec
		}
		return append(b, p[n:]...), nil
	}
	return nil, errCodec
}
func fileList(d modules.AppDefinition, seq byte) []byte {
	b := append([]byte{1}, field(d.Token, seq)...)
	b = append(b, be16(uint16(len(d.Files)))...)
	for _, f := range d.Files {
		b = append(b, be32(uint32(len(f)))...)
	}
	return b
}
func fileChunk(d modules.AppDefinition, f Frame) ([]byte, error) {
	p := f.Payload
	if len(p) != 8 {
		return nil, errCodec
	}
	i := int(binary.BigEndian.Uint16(p))
	off := int(binary.BigEndian.Uint32(p[2:]))
	size := int(binary.BigEndian.Uint16(p[6:]))
	if i >= len(d.Files) || off >= len(d.Files[i]) || size > 8192 {
		return nil, errCodec
	}
	end := off + size
	if end > len(d.Files[i]) {
		end = len(d.Files[i])
	}
	b := append([]byte(nil), p[:6]...)
	return append(b, field(d.Files[i][off:end], f.Sequence)...), nil
}
