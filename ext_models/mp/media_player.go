package ext_mp

type MPlayerPlay struct {
	_msgpack    struct{} `msgpack:",as_array"`
	PlayerIndex int
}
