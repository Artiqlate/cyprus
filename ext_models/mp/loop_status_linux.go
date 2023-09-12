//go:build linux
// +build linux

package ext_mp

import (
	"fmt"

	"github.com/Artiqlate/ganymede/models/mp"
)

func ParseLoopStatus(loopStatus string) (string, error) {
	switch loopStatus {
	case mp.LoopStatusNone, mp.LoopStatusTrack, mp.LoopStatusPlaylist:
		return loopStatus, nil
	default:
		return mp.LoopStatusError, fmt.Errorf(
			"parseLoopStatus(linux): '%s' is not a valid loop status value",
			loopStatus,
		)
	}
}
