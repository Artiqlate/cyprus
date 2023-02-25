package subsystems

import (
	"fmt"
	"runtime"

	media_player "crosine.com/cyprus/subsystems/media_player"
)

type MediaPlayerSubsystem interface {
	Setup() error
	Routine()
}

func NewMediaPlayerSubsystem() (MediaPlayerSubsystem, error) {
	// Only platform currently supported is Linux
	if runtime.GOOS == "linux" {
		return media_player.NewLinuxMediaPlayerSubsystem(), nil
	}
	return nil, fmt.Errorf("MediaPlayerSubsystem: OS not supported (%s)", runtime.GOOS)
}
