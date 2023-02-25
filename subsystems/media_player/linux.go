package media_player

import (
	"bytes"
	"fmt"
	"strings"

	"crosine.com/cyprus/comm"
	"github.com/Pauloo27/go-mpris"
	"github.com/godbus/dbus/v5"
	"github.com/vmihailenco/msgpack/v5"
)

type LinuxMediaPlayerSubsystem struct {
	bus          *dbus.Conn
	bidirChannel *comm.BiDirMessageChannel
}

func NewLinuxMediaPlayerSubsystem(bidirChan *comm.BiDirMessageChannel) *LinuxMediaPlayerSubsystem {
	return &LinuxMediaPlayerSubsystem{
		bidirChannel: bidirChan,
	}
}

func (lmp *LinuxMediaPlayerSubsystem) Setup() error {
	busConn, sessionBussErr := dbus.SessionBus()
	if sessionBussErr != nil {
		return sessionBussErr
	}
	lmp.bus = busConn
	return nil
}

// func NewLinuxMediaPlayerSubsystem(bidirChannel comm.BiDirMessageChannel) (*LinuxMediaPlayerSubsystem, error) {
// 	mp := &LinuxMediaPlayerSubsystem{}

// 	// Get DBus Session Bus
// 	busConn, sessionBusError := dbus.SessionBus()
// 	if sessionBusError != nil {
// 		return nil, sessionBusError
// 	}
// 	// No errors, add the bidirectional channels
// 	mp.bus = busConn
// 	mp.bidirChannel = bidirChannel

// 	// TODO: Add methods to add mpris library and the like
// 	return mp, nil
// }

func (l *LinuxMediaPlayerSubsystem) List() ([]string, error) {
	return mpris.List(l.bus)
}

func (l *LinuxMediaPlayerSubsystem) Routine() {
	fmt.Println("MP: Starting")
	if l.bidirChannel.InChannel == nil || l.bidirChannel.OutChannel == nil {
		return
	}
	for close := false; !close; {
		select {
		case readData := <-l.bidirChannel.InChannel:
			// This read channel will recieve the and will run actions which are deemed required
			decoder := msgpack.NewDecoder(bytes.NewReader(readData))
			methodData, decodeErr := decoder.DecodeString()
			if decodeErr != nil {
				fmt.Printf("Decode error: %v", decodeErr)
			}

			methodWithoutValue, method, methodExists := strings.Cut(methodData, ":")
			if !methodExists {
				fmt.Println("Method doesn't exist!!")
				method = methodWithoutValue
			}
			switch method {
			// TODO: Switch to just "close". Strip the submodule name if exists
			case "mp:close", "close":
				close = true
			default:
			}
		case moduleCommand := <-l.bidirChannel.CommandChannel:
			// If there's any other commands, put here
			switch moduleCommand {
			case "close":
				close = true
			}
		}
	}
	fmt.Println("MP: Stopping")
}

func (l *LinuxMediaPlayerSubsystem) Shutdown() {
	l.bidirChannel.CommandChannel <- "close"
}
