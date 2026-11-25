// Native packet adapter ported from openSNAP nm/native_transport.py, using the
// existing shared NM STUN control contract. No STUN packet is invented for a PS2.
package stun

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type NativeStatus struct {
	Connected   bool   `json:"connected"`
	UsingBackup bool   `json:"using_backup"`
	Received    uint64 `json:"received"`
	Sent        uint64 `json:"sent"`
	LastError   string `json:"last_error,omitempty"`
}
type NativeConn struct {
	conn                         *net.UnixConn
	local                        *net.UDPAddr
	control, backup, path, owner string
	rpcMu                        sync.Mutex
	mu                           sync.RWMutex
	channel                      string
	activeControl                string
	connected                    bool
	lastError                    string
	closed                       bool
	received, sent               atomic.Uint64
	once                         sync.Once
	cancel                       context.CancelFunc
}

func ListenNative(ctx context.Context, control, host string, port int) (*NativeConn, error) {
	return listenNative(ctx, control, "", host, port)
}

func ListenNativeWithBackup(ctx context.Context, control, backup, host string, port int) (*NativeConn, error) {
	return listenNative(ctx, control, backup, host, port)
}

func listenNative(ctx context.Context, control, backup, host string, port int) (*NativeConn, error) {
	if !filepath.IsAbs(control) || port < 1 || port > 65535 {
		return nil, errors.New("native STUN control path and UDP port required")
	}
	if backup != "" && (!filepath.IsAbs(backup) || filepath.Dir(backup) != filepath.Dir(control) || filepath.Clean(backup) == filepath.Clean(control)) {
		return nil, errors.New("backup STUN control must be a distinct socket in the same directory")
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return nil, errors.New("native adapter requires IPv4 bind address")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	owner := hex.EncodeToString(nonce[:])
	path := filepath.Join(filepath.Dir(control), "client-"+owner[:24]+".sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		conn.Close()
		os.Remove(path)
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	n := &NativeConn{conn: conn, local: &net.UDPAddr{IP: ip.To4(), Port: port}, control: control, backup: backup, path: path, owner: owner, cancel: cancel}
	if err = n.reserve(); err != nil {
		n.Close()
		return nil, err
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-child.Done():
				n.Close()
				return
			case <-ticker.C:
				_ = n.reserve()
			}
		}
	}()
	return n, nil
}
func (n *NativeConn) reserve() error {
	n.rpcMu.Lock()
	defer n.rpcMu.Unlock()
	n.mu.RLock()
	closed := n.closed
	first, second := n.control, n.backup
	if n.backup != "" && n.activeControl == n.backup {
		first, second = second, first
	}
	n.mu.RUnlock()
	if closed {
		return net.ErrClosed
	}
	reserveAt := func(control string) (string, error) {
		c, err := net.DialTimeout("unix", control, 150*time.Millisecond)
		if err != nil {
			return "", err
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(150 * time.Millisecond))
		payload := map[string]any{"operation": "reserver", "proprietaire": n.owner, "ip": n.local.IP.String(), "port": n.local.Port, "client": n.path}
		if err = json.NewEncoder(c).Encode(payload); err != nil {
			return "", err
		}
		line, err := bufio.NewReader(io.LimitReader(c, 65537)).ReadBytes('\n')
		if err != nil || len(line) > 65536 {
			return "", errors.New("invalid STUN control response")
		}
		var r struct {
			OK      bool   `json:"ok"`
			Channel string `json:"canal"`
		}
		if json.Unmarshal(line, &r) != nil || !r.OK || filepath.Dir(filepath.Clean(r.Channel)) != filepath.Dir(n.control) || !filepath.IsAbs(r.Channel) {
			return "", errors.New("native STUN reservation refused")
		}
		return r.Channel, nil
	}
	result, err := reserveAt(first)
	if err != nil && second != "" {
		result, err = reserveAt(second)
		first = second
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return net.ErrClosed
	}
	n.connected = err == nil
	if err != nil {
		n.lastError = "control-unavailable"
	} else {
		n.channel = result
		n.activeControl = first
		n.lastError = ""
	}
	return err
}
func (n *NativeConn) ReadFrom(p []byte) (int, net.Addr, error) {
	buf := make([]byte, len(p)+6)
	for {
		size, from, err := n.conn.ReadFromUnix(buf)
		if err != nil {
			return 0, nil, err
		}
		n.mu.RLock()
		channel := n.channel
		n.mu.RUnlock()
		if from == nil || from.Name != channel || size < 6 {
			continue
		}
		port := int(binary.BigEndian.Uint16(buf[4:6]))
		if port == 0 {
			continue
		}
		copied := copy(p, buf[6:size])
		n.received.Add(1)
		return copied, &net.UDPAddr{IP: net.IPv4(buf[0], buf[1], buf[2], buf[3]), Port: port}, nil
	}
}
func (n *NativeConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	to, ok := addr.(*net.UDPAddr)
	if !ok || to.IP.To4() == nil || to.Port < 1 || to.Port > 65535 || len(p) > 65507 {
		return 0, errors.New("invalid native UDP destination or payload")
	}
	buf := make([]byte, len(p)+6)
	copy(buf, to.IP.To4())
	binary.BigEndian.PutUint16(buf[4:], uint16(to.Port))
	copy(buf[6:], p)
	for retry := 0; retry < 2; retry++ {
		n.mu.RLock()
		channel := n.channel
		closed := n.closed
		n.mu.RUnlock()
		if closed {
			return 0, net.ErrClosed
		}
		if channel == "" {
			return 0, errors.New("native transport unavailable")
		}
		count, err := n.conn.WriteToUnix(buf, &net.UnixAddr{Name: channel, Net: "unixgram"})
		if err == nil {
			if count != len(buf) {
				return 0, io.ErrShortWrite
			}
			n.sent.Add(1)
			return len(p), nil
		}
		if retry == 0 && (errors.Is(err, os.ErrNotExist) || isRefused(err)) {
			if err = n.reserve(); err == nil {
				continue
			}
		}
		n.mu.Lock()
		n.connected = false
		n.lastError = "transport-unavailable"
		n.mu.Unlock()
		return 0, err
	}
	return 0, fmt.Errorf("native STUN send failed")
}
func (n *NativeConn) Status() NativeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return NativeStatus{Connected: n.connected, UsingBackup: n.connected && n.backup != "" && n.activeControl == n.backup, Received: n.received.Load(), Sent: n.sent.Load(), LastError: n.lastError}
}
func (n *NativeConn) Close() error {
	var err error
	n.once.Do(func() {
		n.mu.Lock()
		n.closed = true
		n.connected = false
		n.mu.Unlock()
		n.cancel()
		err = n.conn.Close()
		_ = os.Remove(n.path)
	})
	return err
}
func (n *NativeConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: append(net.IP(nil), n.local.IP...), Port: n.local.Port}
}
func (n *NativeConn) SetDeadline(t time.Time) error      { return n.conn.SetDeadline(t) }
func (n *NativeConn) SetReadDeadline(t time.Time) error  { return n.conn.SetReadDeadline(t) }
func (n *NativeConn) SetWriteDeadline(t time.Time) error { return n.conn.SetWriteDeadline(t) }
