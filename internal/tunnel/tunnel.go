package tunnel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/danilbrenner/sshelob/internal/config"
)

const (
	defaultLogCapacity  = 200
	initialRetryBackoff = time.Second
	maxRetryBackoff     = 30 * time.Second
)

type TunnelState string

const (
	Stopped      TunnelState = "stopped"
	Connecting   TunnelState = "connecting"
	Connected    TunnelState = "connected"
	Reconnecting TunnelState = "reconnecting"
	Error        TunnelState = "error"
)

type RingBuffer struct {
	mu       sync.RWMutex
	lines    []string
	start    int
	size     int
	capacity int
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}

	return &RingBuffer{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

func (r *RingBuffer) Add(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size < r.capacity {
		r.lines[(r.start+r.size)%r.capacity] = line
		r.size++
		return
	}

	r.lines[r.start] = line
	r.start = (r.start + 1) % r.capacity
}

func (r *RingBuffer) Lines() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, r.size)
	for i := range r.size {
		result[i] = r.lines[(r.start+i)%r.capacity]
	}
	return result
}

type sshClient interface {
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
	Close() error
}

type dialFunc func(context.Context, config.TunnelDef) (sshClient, error)
type forwardFunc func(context.Context, *Tunnel, sshClient, config.TunnelDef) error
type sleepFunc func(context.Context, time.Duration) error

type Option func(*Tunnel)

func WithLogCapacity(capacity int) Option {
	return func(t *Tunnel) {
		t.logBuffer = NewRingBuffer(capacity)
	}
}

func WithDialFunc(fn dialFunc) Option {
	return func(t *Tunnel) {
		t.dial = fn
	}
}

func WithForwardFunc(fn forwardFunc) Option {
	return func(t *Tunnel) {
		t.forward = fn
	}
}

func WithSleepFunc(fn sleepFunc) Option {
	return func(t *Tunnel) {
		t.sleep = fn
	}
}

func WithEventWriter(writer io.Writer) Option {
	return func(t *Tunnel) {
		t.eventsOut = writer
	}
}

type Tunnel struct {
	def       config.TunnelDef
	logBuffer *RingBuffer
	eventsOut io.Writer

	stateMu sync.RWMutex
	state   TunnelState
	stateCh chan TunnelState

	runMu   sync.Mutex
	running bool
	cancel  context.CancelFunc

	dial    dialFunc
	forward forwardFunc
	sleep   sleepFunc
}

func NewTunnel(def config.TunnelDef, opts ...Option) *Tunnel {
	tunnel := &Tunnel{
		def:       def,
		logBuffer: NewRingBuffer(defaultLogCapacity),
		eventsOut: io.Discard,
		state:     Stopped,
		stateCh:   make(chan TunnelState, 64),
		dial:      defaultDial,
		forward:   defaultForward,
		sleep:     defaultSleep,
	}

	for _, opt := range opts {
		opt(tunnel)
	}

	return tunnel
}

func (t *Tunnel) State() TunnelState {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()
	return t.state
}

func (t *Tunnel) StateChanges() <-chan TunnelState {
	return t.stateCh
}

func (t *Tunnel) Logs() []string {
	return t.logBuffer.Lines()
}

func (t *Tunnel) Start() error {
	t.runMu.Lock()
	if t.running {
		t.runMu.Unlock()
		return fmt.Errorf("tunnel %q is already running", t.def.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.running = true
	t.runMu.Unlock()

	defer func() {
		t.runMu.Lock()
		t.cancel = nil
		t.running = false
		t.runMu.Unlock()
	}()

	return t.run(ctx)
}

func (t *Tunnel) Stop() error {
	t.runMu.Lock()
	cancel := t.cancel
	running := t.running
	t.runMu.Unlock()

	if !running {
		t.setState(Stopped)
		return nil
	}

	cancel()
	t.setState(Stopped)
	t.logf("tunnel %q stopped", t.def.Name)
	return nil
}

func (t *Tunnel) run(ctx context.Context) error {
	backoff := initialRetryBackoff
	attempt := 0

	for {
		if err := ctx.Err(); err != nil {
			t.setState(Stopped)
			return nil
		}

		if attempt == 0 {
			t.setState(Connecting)
		} else {
			t.setState(Reconnecting)
		}

		client, err := t.dial(ctx, t.def)
		if err != nil {
			t.setState(Error)
			t.logf("dial failed: %v", err)
			if !t.waitBackoff(ctx, backoff) {
				t.setState(Stopped)
				return nil
			}
			backoff = nextBackoff(backoff)
			attempt++
			continue
		}

		err = t.forward(ctx, t, client, t.def)
		_ = client.Close()
		if err != nil && ctx.Err() == nil {
			t.setState(Error)
			t.logf("tunnel disconnected: %v", err)
		}

		if err := ctx.Err(); err != nil {
			t.setState(Stopped)
			return nil
		}

		t.logf("reconnecting after disconnect")
		if !t.waitBackoff(ctx, backoff) {
			t.setState(Stopped)
			return nil
		}

		backoff = nextBackoff(backoff)
		attempt++
	}
}

func (t *Tunnel) waitBackoff(ctx context.Context, duration time.Duration) bool {
	if err := t.sleep(ctx, duration); err != nil {
		return false
	}
	return true
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maxRetryBackoff {
		return maxRetryBackoff
	}
	return next
}

func (t *Tunnel) setState(state TunnelState) {
	t.stateMu.Lock()
	if t.state == state {
		t.stateMu.Unlock()
		return
	}
	t.state = state
	t.stateMu.Unlock()

	select {
	case t.stateCh <- state:
	default:
	}
}

func (t *Tunnel) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	t.logBuffer.Add(msg)
	if t.eventsOut == nil {
		return
	}
	_, _ = fmt.Fprintf(t.eventsOut, "[%s] %s\n", t.def.Name, msg)
}

func defaultSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func defaultDial(ctx context.Context, def config.TunnelDef) (sshClient, error) {
	keyPath, err := expandPath(def.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("expand key path: %w", err)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key file %q: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse private key %q: %w", keyPath, err)
	}

	addr := net.JoinHostPort(def.Host, strconv.Itoa(def.Port))
	clientConfig := &ssh.ClientConfig{
		User:            def.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial ssh target %s: %w", addr, err)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("create ssh client connection: %w", err)
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("key_path is required")
	}

	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}

		if path == "~" {
			return home, nil
		}

		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}

	return path, nil
}

func defaultForward(ctx context.Context, t *Tunnel, client sshClient, def config.TunnelDef) error {
	switch def.Type {
	case config.TunnelTypeLocal:
		return runLocalForward(ctx, t, client, def.BindAddr, def.DestAddr)
	case config.TunnelTypeRemote:
		return runRemoteForward(ctx, t, client, def.BindAddr, def.DestAddr)
	case config.TunnelTypeDynamic:
		return runDynamicForward(ctx, t, client, def.BindAddr)
	default:
		return fmt.Errorf("unsupported tunnel type: %q", def.Type)
	}
}

func runLocalForward(ctx context.Context, t *Tunnel, client sshClient, bindAddr, destAddr string) error {
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", bindAddr, err)
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			t.logf("failed to close listener: %v", err)
		}
	}(listener)

	t.setState(Connected)
	t.logf("local forward listening on %s", bindAddr)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept local connection: %w", err)
		}

		go func(localConn net.Conn) {
			defer func(localConn net.Conn) {
				err := localConn.Close()
				if err != nil {
					t.logf("failed to close local connection: %v", err)
				}
			}(localConn)
			remoteConn, dialErr := client.Dial("tcp", destAddr)
			if dialErr != nil {
				t.logf("dial destination %s failed: %v", destAddr, dialErr)
				return
			}
			defer func(remoteConn net.Conn) {
				err := remoteConn.Close()
				if err != nil {
					t.logf("failed to close remote connection: %v", err)
				}
			}(remoteConn)
			pipeConns(localConn, remoteConn)
		}(conn)
	}
}

func runRemoteForward(ctx context.Context, t *Tunnel, client sshClient, bindAddr, destAddr string) error {
	listener, err := client.Listen("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("remote listen on %s: %w", bindAddr, err)
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			t.logf("failed to close listener: %v", err)
		}
	}(listener)

	t.setState(Connected)
	t.logf("remote forward listening on %s", bindAddr)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept remote connection: %w", err)
		}

		go func(src net.Conn) {
			defer func(src net.Conn) {
				err := src.Close()
				if err != nil {
					t.logf("failed to close remote connection: %v", err)
				}
			}(src)
			localConn, dialErr := (&net.Dialer{}).DialContext(ctx, "tcp", destAddr)
			if dialErr != nil {
				t.logf("dial local destination %s failed: %v", destAddr, dialErr)
				return
			}
			defer func(localConn net.Conn) {
				err := localConn.Close()
				if err != nil {
					t.logf("failed to close local connection: %v", err)
				}
			}(localConn)
			pipeConns(src, localConn)
		}(remoteConn)
	}
}

func runDynamicForward(ctx context.Context, t *Tunnel, client sshClient, bindAddr string) error {
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", bindAddr, err)
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			t.logf("failed to close listener: %v", err)
		}
	}(listener)

	t.setState(Connected)
	t.logf("dynamic forward listening on %s", bindAddr)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept socks5 connection: %w", err)
		}

		go t.handleSOCKS5Conn(client, conn)
	}
}

func (t *Tunnel) handleSOCKS5Conn(client sshClient, conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			t.logf("failed to close connection: %v", err)
		}
	}(conn)

	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != 0x05 {
		return
	}

	nMethods := int(header[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}

	noAuthSupported := false
	for _, method := range methods {
		if method == 0x00 {
			noAuthSupported = true
			break
		}
	}

	if !noAuthSupported {
		_, _ = conn.Write([]byte{0x05, 0xff})
		return
	}

	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return
	}

	if reqHeader[0] != 0x05 {
		return
	}

	if reqHeader[1] != 0x01 {
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	destHost, err := readSOCKS5Host(conn, reqHeader[3])
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return
	}
	destPort := int(binary.BigEndian.Uint16(portBytes))
	destAddr := net.JoinHostPort(destHost, strconv.Itoa(destPort))

	targetConn, err := client.Dial("tcp", destAddr)
	if err != nil {
		t.logf("socks5 dial %s failed: %v", destAddr, err)
		_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer func(targetConn net.Conn) {
		err := targetConn.Close()
		if err != nil {
			t.logf("socks5 dial %s failed: %v", destAddr, err)
		}
	}(targetConn)

	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	pipeConns(conn, targetConn)
}

func readSOCKS5Host(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		addr := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", err
		}
		return net.IP(addr).String(), nil
	case 0x03:
		size := make([]byte, 1)
		if _, err := io.ReadFull(conn, size); err != nil {
			return "", err
		}
		host := make([]byte, int(size[0]))
		if _, err := io.ReadFull(conn, host); err != nil {
			return "", err
		}
		return string(host), nil
	case 0x04:
		addr := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", err
		}
		return net.IP(addr).String(), nil
	default:
		return "", fmt.Errorf("unsupported atyp: %d", atyp)
	}
}

func pipeConns(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		closeWrite(a)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		closeWrite(b)
	}()

	wg.Wait()
}

func closeWrite(conn net.Conn) {
	type closeWriter interface {
		CloseWrite() error
	}

	if cw, ok := conn.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = conn.Close()
}
