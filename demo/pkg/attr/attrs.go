package attr

import (
	"fmt"
	"log/slog"

	"github.com/pion/webrtc/v4"
)

func Error(err error) slog.Attr {
	return slog.Any("error", err)
}

func State[T fmt.Stringer](state T) slog.Attr {
	return slog.String("state", state.String())
}

func SSRC(ssrc webrtc.SSRC) slog.Attr {
	return slog.Uint64("ssrc", uint64(ssrc))
}

func Mid(mid string) slog.Attr {
	return slog.String("mid", mid)
}
