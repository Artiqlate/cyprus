package media_player

import "fmt"

type PlatformKind string

const (
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	PlatformMacOS   = "macos"
)

var Platforms = map[PlatformKind]bool{PlatformLinux: false, PlatformMacOS: false, PlatformWindows: false}

func GenerateMethod(module string, platform PlatformKind, method string) string {
	if _, platformExists := Platforms[platform]; platformExists {
		return fmt.Sprintf("%s:%s:%s", module, platform, method)
	}
	return fmt.Sprintf("%s:%s", module, method)
}
