package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// echoServer simula o servidor VNC.
func echoServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(c, c); c.Close() }()
		}
	}()
	return l.Addr().String()
}

func roundTrip(t *testing.T, addr, msg string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(c, "%s\n", msg)
	got, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		t.Fatalf("leitura: %v", err)
	}
	if got != msg+"\n" {
		t.Fatalf("got %q, want %q", got, msg)
	}
}

func TestRelayEndToEnd(t *testing.T) {
	vnc := echoServer(t)
	clientAddr, hostAddr := freeAddr(t), freeAddr(t)
	stop := make(chan struct{})
	defer close(stop)

	// Host sobe antes do device: deve tentar de novo até conseguir.
	go runHost(hostAddr, vnc, 2, stop)
	time.Sleep(200 * time.Millisecond)

	ready := make(chan struct{})
	go func() { _ = runDevice(clientAddr, hostAddr, 5*time.Second, ready) }()
	<-ready

	// Várias sessões em sequência e em paralelo.
	for i := 0; i < 3; i++ {
		roundTrip(t, clientAddr, fmt.Sprintf("seq-%d", i))
	}
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			roundTrip(t, clientAddr, fmt.Sprintf("par-%d", i))
		}(i)
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestDeviceDropsDeadHostConns(t *testing.T) {
	c1, c2 := net.Pipe()
	_ = c2.Close()
	ch := make(chan net.Conn, 1)
	ch <- c1
	if takeAlive(ch, 100*time.Millisecond) != nil {
		t.Fatal("conexão morta não deveria ser usada")
	}
}
