package conn_pool

import (
	"context"
	"testing"
	"time"
)

func TestConn_New(t *testing.T) {
	ctx := context.Background()
	config := Config{MaxConn: 2, MaxIdle: 1}
	conn := Prepare(ctx, &config)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	if _, err := conn.New(ctx); err != nil {
		return
	}
	if _, err := conn.New(ctx); err != nil {
		// err = timeout
		return
	}
}

func TestConn_Release(t *testing.T) {
	ctx := context.Background()
	config := Config{MaxConn: 2, MaxIdle: 1}
	conn := Prepare(ctx, &config)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	if _, err := conn.New(ctx); err != nil {
		return
	}
	go func() {
		time.Sleep(time.Second)
		conn.Release(ctx)
	}()
	// receive released conn
	if _, err := conn.New(ctx); err != nil {
		return
	}
	// err = timeout
	if _, err := conn.New(ctx); err != nil {
		return
	}
}

func TestConn_Release2(t *testing.T) {
	ctx := context.Background()
	config := Config{MaxConn: 2, MaxIdle: 1}
	conn := Prepare(ctx, &config)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	go conn.Release(ctx)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	go conn.Release(ctx)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	go conn.Release(ctx)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	go conn.Release(ctx)
	if _, err := conn.New(ctx); err != nil {
		return
	}
	go conn.Release(ctx)
	if _, err := conn.New(ctx); err != nil {
		return
	}
}
