package media_player

import "crosine.com/cyprus/utils"

const (
	MediaPlayerSubsystemName = "mp"
)

func MPAutoPlatformMethod(method string) string {
	return utils.GenerateAutoPlatformMethod(MediaPlayerSubsystemName, method)
}

func MPMethod(method string) string {
	return utils.GenerateMethod(MediaPlayerSubsystemName, method)
}
