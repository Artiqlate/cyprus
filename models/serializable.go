package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack/v5"
)

type ISerializable interface {
	Encode() ([]byte, error)
}

type Serializable struct {
	// _msgpack  struct{} `msgpack:",as_array"`
	Type      int8
	Id        uuid.UUID
	Timestamp time.Time
}

func NewSerializable() (*Serializable, error) {
	id_val, id_err := uuid.NewRandom()
	if id_err != nil {
		return nil, id_err
	}
	return &Serializable{
		Id:        id_val,
		Timestamp: time.Now(),
	}, nil
}

// -- GETTERS AND SETTERS

func (ser *Serializable) GetType() int8 {
	return ser.Type
}

func (ser *Serializable) GetId() uuid.UUID {
	return ser.Id
}

func (ser *Serializable) GetTimestamp() time.Time {
	return ser.Timestamp
}

// -- METHODS

func (object_to_encode *Serializable) Encode() ([]byte, error) {
	if object_to_encode == nil {
		return nil, errors.New("serializable object marshaling error")
	}
	marshalled, err := msgpack.Marshal(object_to_encode)
	if err != nil {
		return nil, err
	}
	return marshalled, nil
}

func DecodeSerializable(data []byte) (*Serializable, error) {
	if data == nil {
		return nil, errors.New("null data")
	}
	var serialized Serializable
	marshal_err := msgpack.Unmarshal(data, &serialized)
	if marshal_err != nil {
		return nil, marshal_err
	}
	return &serialized, nil
}
