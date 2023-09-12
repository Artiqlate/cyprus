package media_player

import "github.com/Artiqlate/cyprus/utils"

const (
	MediaPlayerSubsystemName = "mp"
)

func MPAutoPlatformMethod(method string) string {
	return utils.GenerateAutoPlatformMethod(MediaPlayerSubsystemName, method)
}

func MPMethod(method string) string {
	return utils.GenerateMethod(MediaPlayerSubsystemName, method)
}
