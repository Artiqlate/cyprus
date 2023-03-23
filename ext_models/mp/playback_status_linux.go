//go:build linux
// +build linux

package ext_mp

/*
PlaybackStatus (Linux)

This parses media playback status values for Linux-based Operating Systems.

Ref: https://specifications.freedesktop.org/mpris-spec/latest/Player_Interface.html#Enum:Playback_Status

Copyright (C) 2023 Goutham Krishna K V
*/

import (
	"fmt"

	"github.com/CrosineEnterprises/ganymede/models/mp"
)

func ParsePlaybackStatus(playbackStatus string) (string, error) {
	switch playbackStatus {
	case mp.PlaybackStatusPlaying, mp.PlaybackStatusPaused, mp.PlaybackStatusStopped:
		return playbackStatus, nil
	default:
		return mp.PlaybackStatusError, fmt.Errorf(
			"playbackStatus(linux): %s is not a proper playback value",
			playbackStatus,
		)
	}
}
