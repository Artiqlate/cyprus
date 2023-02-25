package comm

import "github.com/CrosineEnterprises/ganymede/models"

type BiDirMessageChannel struct {
	InChannel  chan []byte
	OutChannel chan models.Message
}

func NewBiDirMessageChannel() *BiDirMessageChannel {
	return &BiDirMessageChannel{
		InChannel:  make(chan []byte),
		OutChannel: make(chan models.Message),
	}
}
