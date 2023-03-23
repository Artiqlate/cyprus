//go:build windows

package ext_mp

/*
PlaybackStatus (Windows)

This parses media playback status values for Windows-based Operating Systems.

Ref: https://learn.microsoft.com/en-us/uwp/api/windows.media.mediaplaybackstatus#fields

Copyright (C) 2023 Goutham Krishna K V
*/

func ParsePlaybackStatus(playbackStatus string) (string, error) {
	convertedStatus, convErr := strconv.ParseInt(playbackStatus, 10, 8)
	if convErr != nil {
		return PlaybackStatusError, fmt.Errorf(
			"playbackStatus: <WIN> playback status '%s' "+
				"cannot be converted: %v", statusValue, convErr,
		)
	}
	switch convertedStatus {
	case 0:
		return PlaybackStatusClosed, nil
	case 1:
		return PlaybackStatusChanging, nil
	case 2:
		return PlaybackStatusStopped, nil
	case 3:
		return PlaybackStatusPlaying, nil
	case 4:
		return PlaybackStatusPaused, nil
	default:
		return PlaybackStatusError, fmt.Errorf(
			"playbackStatus(win): %d is not a proper playback value",
			statusValue,
		)
	}
}
