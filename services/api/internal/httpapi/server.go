package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

func Run(ctx context.Context, server *http.Server, grace time.Duration) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	return serve(ctx, server, listener, grace)
}

// Serve stops accepting connections before Shutdown finishes draining them.
// The caller must wait for both operations before the process can exit.
func serve(ctx context.Context, server *http.Server, listener net.Listener, grace time.Duration) error {
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	select {
	case err := <-finished:
		return servingError(err)
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		upgraded := make(chan error, 1)
		go func() {
			if handler, ok := server.Handler.(interface{ Shutdown(context.Context) error }); ok {
				upgraded <- handler.Shutdown(shutdown)
			} else {
				upgraded <- nil
			}
		}()
		err := server.Shutdown(shutdown)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		return errors.Join(err, <-upgraded, servingError(<-finished))
	}
}

func servingError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
