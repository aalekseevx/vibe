package main

import (
	"fmt"
	"log/slog"
)

func ErrAttr(err error) slog.Attr {
	return slog.Any("error", err)
}

func StateAttr[T fmt.Stringer](state T) slog.Attr {
	return slog.Any("state", state.String())
}
