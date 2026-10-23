package httpapi

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownWaitsForActiveRequest(t *testing.T) {
	entered, release, stopping := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("completed"))
	})}
	server.RegisterOnShutdown(func() { close(stopping) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener, time.Second) }()
	response := make(chan string, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		r, e := client.Get("http://" + listener.Addr().String())
		if e != nil {
			response <- e.Error()
			return
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		response <- string(body)
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-stopping:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("returned before active request completed: %v", err)
	default:
	}
	close(release)
	if body := <-response; body != "completed" {
		t.Fatalf("response: %s", body)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
}

func TestShutdownDeadlineClosesConnection(t *testing.T) {
	entered, exited := make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(exited)
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done, requestDone := make(chan error, 1), make(chan error, 1)
	go func() { done <- serve(ctx, server, listener, 30*time.Millisecond) }()
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		r, e := client.Get("http://" + listener.Addr().String())
		if r != nil {
			r.Body.Close()
		}
		requestDone <- e
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown stuck")
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("handler connection was not closed")
	}
	if err := <-requestDone; err == nil {
		t.Fatal("request unexpectedly completed")
	}
}

func TestStartupFailureReturnsWithoutWaitingForSignal(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), &http.Server{Addr: listener.Addr().String()}, time.Second)
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected address-in-use error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("startup waited for a signal")
	}
}
