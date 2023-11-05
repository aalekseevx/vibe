package main

import (
	"fmt"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

type PeerConnectionFactory struct {
	api       *webrtc.API
	iceServer string
}

func newPeerConnectionFactory(config Config) (PeerConnectionFactory, error) {
	m := &webrtc.MediaEngine{}
	err := m.RegisterDefaultCodecs()
	if err != nil {
		return PeerConnectionFactory{}, fmt.Errorf("register default codecs: %w", err)
	}

	ir := &interceptor.Registry{}
	err = webrtc.RegisterDefaultInterceptors(m, ir)
	if err != nil {
		return PeerConnectionFactory{}, fmt.Errorf("register default interceptors: %w", err)
	}

	return PeerConnectionFactory{
		api:       webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(ir)),
		iceServer: config.IceServer,
	}, nil
}

func (f PeerConnectionFactory) New() (*webrtc.PeerConnection, error) {
	pc, err := f.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{f.iceServer},
			},
		},
	})
	return pc, err
}
