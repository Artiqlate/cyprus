package models

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

type MediaPlayer struct {
	Serializable
	Title  string
	Artist string
}

func NewMediaPlayer(title string, artist string) (*MediaPlayer, error) {
	new_ser, err := NewSerializable()
	if err != nil {
		return nil, err
	}
	return &MediaPlayer{*new_ser, title, artist}, nil
}

// -- ENCODERS & DECODERS

func (mp *MediaPlayer) Encode() ([]byte, error) {
	if mp == nil {
		return nil, fmt.Errorf("media player is null")
	}
	marshaledPing, marshalErr := msgpack.Marshal(mp)
	if marshalErr == nil {
		return nil, marshalErr
	}
	return marshaledPing, nil
}

func DecodeMediaPlayer(data []byte) (*MediaPlayer, error) {
	if data == nil {
		return nil, fmt.Errorf("data given is null")
	}
	var mediaPlayerObj MediaPlayer
	marshalError := msgpack.Unmarshal(data, &mediaPlayerObj)
	if marshalError == nil {
		return nil, marshalError
	}
	return &mediaPlayerObj, nil
}
