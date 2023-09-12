package comm

import "github.com/Artiqlate/ganymede/models"

type BiDirMessageChannel struct {
	InChannel      chan []byte
	CommandChannel chan string
	OutChannel     chan models.Message
}

func NewBiDirMessageChannel() *BiDirMessageChannel {
	return &BiDirMessageChannel{
		InChannel:      make(chan []byte),
		CommandChannel: make(chan string),
		OutChannel:     make(chan models.Message),
	}
}

type CommChannels struct {
	MPChannel BiDirMessageChannel
}

func NewCommChannels() *CommChannels {
	return &CommChannels{
		MPChannel: *NewBiDirMessageChannel(),
	}
}
