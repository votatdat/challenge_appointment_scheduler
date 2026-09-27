package postgres

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestOpenBoundsStalledHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	finished := make(chan error, 1)
	go func() {
		pool, err := Open(t.Context(), fmt.Sprintf("postgres://test:test@%s/test?sslmode=disable", listener.Addr()), 1, 100*time.Millisecond)
		if pool != nil {
			pool.Close()
		}
		finished <- err
	}()
	var conn net.Conn
	select {
	case conn = <-accepted:
		// Accept TCP but never answer the PostgreSQL startup handshake.
		defer conn.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("database handshake did not start")
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("stalled handshake unexpectedly succeeded")
		}
	case <-time.After(2 * time.Second):
		// Unblock cleanup even if connection creation ignored its deadline.
		conn.Close()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("connection worker did not stop after disconnect")
		}
		t.Fatal("Open exceeded its connection deadline while cleaning up the pool")
	}
}
