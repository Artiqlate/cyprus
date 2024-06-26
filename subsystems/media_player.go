package subsystems

import (
	"fmt"
	"runtime"

	"github.com/Artiqlate/cyprus/comm"
	media_player "github.com/Artiqlate/cyprus/subsystems/media_player"
)

type MediaPlayerSubsystem interface {
	// -- SUBSYSTEM METHODS --
	Setup() error
	Routine()
	Shutdown()
	// -- MEDIA PLAYER - SPECIFIC METHODS
	// ListPlayers() ([]string, error)
	// GetPlayers() error
}

func NewMediaPlayerSubsystem(bidirChan *comm.BiDirMessageChannel) (MediaPlayerSubsystem, error) {
	// Only platform currently supported is Linux
	if runtime.GOOS == "linux" {
		return media_player.NewLinuxMediaPlayerSubsystem(bidirChan), nil
	}
	return nil, fmt.Errorf("MediaPlayerSubsystem: OS not supported (%s)", runtime.GOOS)
}
