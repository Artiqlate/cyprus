package ext_models

import (
	models_base "github.com/CrosineEnterprises/ganymede/models/base"
	"github.com/vmihailenco/msgpack/v5"
	"nhooyr.io/websocket"
)

func ProcessPing(conn *websocket.Conn, decoder *msgpack.Decoder) (*models_base.Ping, error) {
	ping, decodeErr := models_base.DecodePing(decoder)
	if decodeErr != nil {
		return nil, decodeErr
	}
	// WE DO NOT RETURN THIS. PROCESS THIS VALUE!!
	return ping, nil
}
