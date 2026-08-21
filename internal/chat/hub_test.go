package chat

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestHubCloseAllClosesRegisteredConnections(t *testing.T) {
	hub := NewHub()

	serverClosed := make(chan struct{}, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)

		hub.register(c)
		defer hub.unregister(c)

		_, _, err = c.Read(context.Background())
		if websocket.CloseStatus(err) != -1 || errors.Is(err, net.ErrClosed) {
			serverClosed <- struct{}{}
		}
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	dial := func() *websocket.Conn {
		c, _, err := websocket.Dial(context.Background(), url, nil)
		require.NoError(t, err)
		return c
	}
	c1 := dial()
	defer c1.CloseNow()
	c2 := dial()
	defer c2.CloseNow()

	readResult := make(chan error, 2)
	read := func(c *websocket.Conn) {
		_, _, err := c.Read(context.Background())
		readResult <- err
	}
	go read(c1)
	go read(c2)

	require.Eventually(t, func() bool {
		return hub.size() == 2
	}, time.Second, 10*time.Millisecond)

	hub.CloseAll("shutting down")

	for i := 0; i < 2; i++ {
		err := <-readResult
		require.Equal(t, websocket.StatusGoingAway, websocket.CloseStatus(err))
	}
	for i := 0; i < 2; i++ {
		select {
		case <-serverClosed:
		case <-time.After(5 * time.Second):
			t.Fatal("server handler did not exit after CloseAll")
		}
	}

	require.Eventually(t, func() bool {
		return hub.size() == 0
	}, time.Second, 10*time.Millisecond)
}
