package media_player

import "crosine.com/cyprus/utils"

const (
	MediaPlayerSubsystemName = "mp"
)

func MPAutoMethod(method string) string {
	return utils.GenerateAutoMethod(MediaPlayerSubsystemName, method)
}

func MPMethod(method string) string {
	return utils.GenerateMethod(MediaPlayerSubsystemName, method)
}
