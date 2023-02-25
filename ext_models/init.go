package ext_models

import (
	models_base "github.com/CrosineEnterprises/ganymede/models/base"
	"github.com/vmihailenco/msgpack/v5"
	"nhooyr.io/websocket"
)

func ProcessInit(conn *websocket.Conn, decoder *msgpack.Decoder) (*models_base.Init, error) {
	ping, decodeErr := models_base.DecodeInit(decoder)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return ping, nil
}
