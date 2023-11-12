package main

import (
	"bwe/demo/pkg/attr"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

type StreamStats struct {
	Timestamp                   int64   `json:"timestamp"`
	SSRC                        uint32  `json:"ssrc"`
	PacketsReceived             uint64  `json:"packets_received"`
	PacketsLost                 int64   `json:"packets_lost"`
	Jitter                      float64 `json:"jitter"`
	LastPacketReceivedTimestamp int64   `json:"last_packet_received_timestamp"`
	HeaderBytesReceived         uint64  `json:"header_bytes_received"`
	BytesReceived               uint64  `json:"bytes_received"`
	FIRCount                    uint32  `json:"fir_count"`
	PLICount                    uint32  `json:"pli_count"`
	NACKCount                   uint32  `json:"nack_count"`
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		slog.Error("load config", attr.Error(err))
		return
	}

	wsConfig, err := websocket.NewConfig(config.Endpoint, "http://localhost")
	if err != nil {
		slog.Error("new ws config", attr.Error(err))
		return
	}

	ws, err := websocket.DialConfig(wsConfig)
	if err != nil {
		slog.Error("dial ws", attr.Error(err))
		return
	}

	m := &webrtc.MediaEngine{}
	err = m.RegisterDefaultCodecs()
	if err != nil {
		slog.Error("register default codecs", attr.Error(err))
		return
	}

	ir := &interceptor.Registry{}
	err = webrtc.RegisterDefaultInterceptors(m, ir)
	if err != nil {
		slog.Error("register default interceptors", attr.Error(err))
		return
	}

	si, err := stats.NewInterceptor()
	if err != nil {
		slog.Error("new stats interceptor", attr.Error(err))
	}

	var statsGetter stats.Getter
	si.OnNewPeerConnection(func(s string, getter stats.Getter) {
		statsGetter = getter
	})

	ir.Add(si)

	se := webrtc.SettingEngine{}
	se.LoggerFactory = logging.NewDefaultLoggerFactory()

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(ir), webrtc.WithSettingEngine(se))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{config.IceServer},
			},
		},
	})

	pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		slog.Info("signaling state changed", attr.State(state))
	})

	pc.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		slog.Info("ice gathering state changed", attr.State(state))
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		slog.Info("ice connection state changed", attr.State(state))
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("connection state changed", attr.State(state))

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			return
		}
	})

	var ssrcMutex sync.Mutex
	ssrcMap := make(map[webrtc.SSRC]struct{}, 0)

	pc.OnTrack(func(remote *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		ssrc := remote.SSRC()
		mid := receiver.RTPTransceiver().Mid()

		slog.Info("track opened", attr.SSRC(ssrc), attr.Mid(mid))

		ssrcMutex.Lock()
		ssrcMap[ssrc] = struct{}{}
		ssrcMutex.Unlock()

		defer func() {
			slog.Info("track closed", attr.SSRC(ssrc), attr.Mid(mid))

			ssrcMutex.Lock()
			delete(ssrcMap, ssrc)
			ssrcMutex.Unlock()
		}()

		buffer := make([]byte, 8000)
		for {
			_, _, err = remote.Read(buffer)
			if err != nil {
				slog.Error("read remote track", attr.Error(err))
				return
			}
		}
	})

	statsFile, err := os.Create(config.ReportFile)
	if err != nil {
		slog.Error("create stats file", attr.Error(err))
		return
	}
	statsEncoder := json.NewEncoder(statsFile)

	go func() {
		ticker := time.NewTicker(config.ReportInterval)
		for range ticker.C {
			ssrcMutex.Lock()
			for ssrc := range ssrcMap {
				rawStats := statsGetter.Get(uint32(ssrc))
				if rawStats == nil {
					slog.Error("stats not found", attr.SSRC(ssrc))
					continue
				}
				stats := StreamStats{
					Timestamp:                   time.Now().UnixNano(),
					SSRC:                        uint32(ssrc),
					PacketsReceived:             rawStats.InboundRTPStreamStats.PacketsReceived,
					PacketsLost:                 rawStats.InboundRTPStreamStats.PacketsLost,
					Jitter:                      rawStats.InboundRTPStreamStats.Jitter,
					LastPacketReceivedTimestamp: rawStats.LastPacketReceivedTimestamp.UnixNano(),
					HeaderBytesReceived:         rawStats.HeaderBytesReceived,
					BytesReceived:               rawStats.BytesReceived,
					FIRCount:                    rawStats.InboundRTPStreamStats.FIRCount,
					PLICount:                    rawStats.InboundRTPStreamStats.PLICount,
					NACKCount:                   rawStats.InboundRTPStreamStats.NACKCount,
				}
				err = statsEncoder.Encode(stats)
				if err != nil {
					slog.Error("encode json", attr.Error(err))
					continue
				}
			}
			ssrcMutex.Unlock()
		}
	}()

	var offer webrtc.SessionDescription
	err = websocket.JSON.Receive(ws, &offer)
	if err != nil {
		slog.Error("receive offer", attr.Error(err))
		return
	}

	err = pc.SetRemoteDescription(offer)
	if err != nil {
		slog.Error("set remote description", attr.Error(err))
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		slog.Error("create answer", attr.Error(err))
		return
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		slog.Error("set local description", attr.Error(err))
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	err = websocket.JSON.Send(ws, pc.LocalDescription())
	if err != nil {
		slog.Error("send local description", attr.Error(err))
		return
	}

	time.Sleep(config.SessionDuration)

	err = ws.Close()
	if err != nil {
		slog.Error("close ws", attr.Error(err))
		return
	}
}
