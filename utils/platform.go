package utils

import (
	"fmt"
	"log"
	"runtime"
)

type PlatformKind string

const (
	PlatformWindows = "windows"
	PlatformLinux   = "linux"
	PlatformMacOS   = "macos"
	PlatformOther   = "other"
)

var Platforms = map[PlatformKind]bool{
	PlatformLinux:   false,
	PlatformMacOS:   false,
	PlatformWindows: false,
	PlatformOther:   false,
}

func GenerateMethod(module string, method string) string {
	return fmt.Sprintf("%s:%s", module, method)
}

func GeneratePlatformMethod(module string, platform PlatformKind, method string) string {
	if _, platformExists := Platforms[platform]; platformExists {
		return fmt.Sprintf("%s:%s:%s", module, platform, method)
	}
	return fmt.Sprintf("%s:%s", module, method)
}

func GenerateAutoPlatformMethod(module string, method string) string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s:%s:%s", module, PlatformWindows, method)
	case "linux":
		return fmt.Sprintf("%s:%s:%s", module, PlatformLinux, method)
	case "darwin":
		return fmt.Sprintf("%s:%s:%s", module, PlatformMacOS, method)
	default:
		log.Printf("WARN[utils/platform]: unsupported platform running")
		return fmt.Sprintf("%s:%s:%s", module, PlatformOther, method)
	}
}
