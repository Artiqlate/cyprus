package models

import (
	"errors"

	"github.com/vmihailenco/msgpack/v5"
)

type Ping struct {
	Serializable
	Message string
}

func NewPing(message string) (*Ping, error) {
	baseSerializable, constructError := NewSerializable()
	if constructError != nil {
		return nil, constructError
	}
	return &Ping{*baseSerializable, message}, nil
}

// -- GETTERS & SETTERS

func (p *Ping) GetMessage() string {
	return p.Message
}

// -- ENCODERS & DECODERS

func (pingToEncode *Ping) Encode() ([]byte, error) {
	if pingToEncode == nil {
		return nil, errors.New("serializable object marshaling error")
	}
	marshaledPing, marshalErr := msgpack.Marshal(pingToEncode)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return marshaledPing, nil
}

func DecodePing(data []byte) (*Ping, error) {
	if data == nil {
		return nil, errors.New("null data")
	}
	var serializedPing Ping
	marshalErr := msgpack.Unmarshal(data, &serializedPing)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return &serializedPing, nil
}
