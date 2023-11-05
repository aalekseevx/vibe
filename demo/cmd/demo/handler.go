package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
	"golang.org/x/net/websocket"
)

type Handler struct {
	PeerConnectionFactory PeerConnectionFactory
	VideoPaths            []string
}

func (h Handler) Watch(ws *websocket.Conn) {
	pc, err := h.PeerConnectionFactory.New()
	if err != nil {
		slog.Error("new peer connection", ErrAttr(err))
		return
	}

	defer func() {
		err = pc.Close()
		if err != nil {
			slog.Error("close peer connection", ErrAttr(err))
		}
	}()

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	for _, videoFileName := range h.VideoPaths {
		err = startTrack(pc, videoFileName, iceConnectedCtx)
		if err != nil {
			slog.Error("start track", ErrAttr(err))
		}
	}

	pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		slog.Info("signaling state changed", StateAttr(state))
	})

	pc.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		slog.Info("ice gathering state changed", StateAttr(state))
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		slog.Info("ice connection state changed", StateAttr(state))
		if state == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("connection state changed", StateAttr(state))

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			return
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		slog.Error("create offer", ErrAttr(err))
		return
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		slog.Error("set local description", ErrAttr(err))
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	err = websocket.JSON.Send(ws, pc.LocalDescription())
	if err != nil {
		slog.Error("send local description", ErrAttr(err))
		return
	}

	var answer webrtc.SessionDescription
	err = websocket.JSON.Receive(ws, &answer)
	if err != nil {
		slog.Error("receive remote description", ErrAttr(err))
		return
	}

	err = pc.SetRemoteDescription(answer)
	if err != nil {
		slog.Error("set remote description", ErrAttr(err))
		return
	}

	websocket.JSON.Receive(ws, nil)
}

func startTrack(pc *webrtc.PeerConnection, videoPath string, iceConnectedCtx context.Context) error {
	file, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	ivf, header, err := ivfreader.NewWith(file)
	if err != nil {
		return fmt.Errorf("new reader: %w", err)
	}

	var trackCodec string
	switch header.FourCC {
	case "AV01":
		trackCodec = webrtc.MimeTypeAV1
	case "VP90":
		trackCodec = webrtc.MimeTypeVP9
	case "VP80":
		trackCodec = webrtc.MimeTypeVP8
	default:
		return fmt.Errorf("unable to handle FourCC %s", header.FourCC)
	}

	videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: trackCodec}, "video", "pion")
	if videoTrackErr != nil {
		return fmt.Errorf("new track local: %w", err)
	}

	rtpSender, videoTrackErr := pc.AddTrack(videoTrack)
	if videoTrackErr != nil {
		return fmt.Errorf("add track: %w", err)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	go func() {
		<-iceConnectedCtx.Done()
		ticker := time.NewTicker(time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000))
		for ; true; <-ticker.C {
			frame, _, ivfErr := ivf.ParseNextFrame()
			if errors.Is(ivfErr, io.EOF) {
				slog.Info(fmt.Sprintf("track %s over", videoPath))
				return
			}

			if ivfErr != nil {
				slog.Error("read ivf", ErrAttr(err))
				return
			}

			if ivfErr = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
				slog.Error("write sample", ErrAttr(err))
				return
			}
		}
	}()

	return nil
}
