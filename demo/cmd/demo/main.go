package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/net/websocket"
)

func main() {
	config, err := LoadConfig()
	if err != nil {
		slog.Error("load config", ErrAttr(err))
		return
	}

	pcFactory, err := newPeerConnectionFactory(config)
	if err != nil {
		slog.Error("new peer connection factory", ErrAttr(err))
		return
	}
	handler := Handler{
		PeerConnectionFactory: pcFactory,
		VideoPaths:            config.VideoPaths,
	}

	http.Handle("/watch", websocket.Handler(handler.Watch))

	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
	if err != nil {
		slog.Error("listen and serve", ErrAttr(err))
		return
	}
}
