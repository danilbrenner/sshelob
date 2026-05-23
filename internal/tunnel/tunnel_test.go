package tunnel

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/danilbrenner/sshelob/internal/config"
)

func TestTunnelStartStopStateTransitions(t *testing.T) {
	tunnelDef := config.TunnelDef{
		Name:     "test",
		Type:     config.TunnelTypeLocal,
		Host:     "example.com",
		User:     "user",
		Port:     22,
		BindAddr: "127.0.0.1:10000",
		DestAddr: "localhost:80",
		KeyPath:  "~/.ssh/id_ed25519",
	}

	dialStarted := make(chan struct{})
	allowDial := make(chan struct{})

	tun := NewTunnel(
		tunnelDef,
		WithDialFunc(func(context.Context, config.TunnelDef) (sshClient, error) {
			close(dialStarted)
			<-allowDial
			return &fakeSSHClient{}, nil
		}),
		WithForwardFunc(func(ctx context.Context, tnl *Tunnel, _ sshClient, _ config.TunnelDef) error {
			tnl.setState(Connected)
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	startErr := make(chan error, 1)
	go func() {
		startErr <- tun.Start()
	}()

	<-dialStarted
	assertStateEventually(t, tun, Connecting)
	close(allowDial)

	assertStateEventually(t, tun, Connected)

	if err := tun.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if err := <-startErr; err != nil {
		t.Fatalf("start returned error: %v", err)
	}

	assertStateEventually(t, tun, Stopped)
}

func TestTunnelReconnectBackoff(t *testing.T) {
	tunnelDef := config.TunnelDef{
		Name:     "backoff-test",
		Type:     config.TunnelTypeLocal,
		Host:     "example.com",
		User:     "user",
		Port:     22,
		BindAddr: "127.0.0.1:10001",
		DestAddr: "localhost:80",
		KeyPath:  "~/.ssh/id_ed25519",
	}

	var (
		mu       sync.Mutex
		recorded []time.Duration
		tun      *Tunnel
	)

	tun = NewTunnel(
		tunnelDef,
		WithDialFunc(func(context.Context, config.TunnelDef) (sshClient, error) {
			return nil, errors.New("dial failed")
		}),
		WithSleepFunc(func(ctx context.Context, delay time.Duration) error {
			mu.Lock()
			recorded = append(recorded, delay)
			count := len(recorded)
			mu.Unlock()

			if count == 5 {
				_ = tun.Stop()
				return ctx.Err()
			}
			return nil
		}),
	)

	if err := tun.Start(); err != nil {
		t.Fatalf("start returned error: %v", err)
	}

	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}

	mu.Lock()
	got := append([]time.Duration(nil), recorded...)
	mu.Unlock()

	if len(got) != len(want) {
		t.Fatalf("backoff samples: got %d, want %d (%v)", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backoff[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRingBufferWraps(t *testing.T) {
	buffer := NewRingBuffer(3)
	buffer.Add("line-1")
	buffer.Add("line-2")
	buffer.Add("line-3")
	buffer.Add("line-4")
	buffer.Add("line-5")

	got := buffer.Lines()
	want := []string{"line-3", "line-4", "line-5"}

	if len(got) != len(want) {
		t.Fatalf("lines len: got %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func assertStateEventually(t *testing.T, tunnel *Tunnel, state TunnelState) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if tunnel.State() == state {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("state did not reach %q, got %q", state, tunnel.State())
}

type fakeSSHClient struct{}

func (f *fakeSSHClient) Dial(string, string) (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSSHClient) Listen(string, string) (net.Listener, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSSHClient) Close() error {
	return nil
}
