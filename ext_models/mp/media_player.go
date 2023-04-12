package ext_mp

// TODO: Change from using index values, to player names. That is more reliable
// and more useful as a unique MPRIS value.
type MPlayerPlay struct {
	_msgpack    struct{} `msgpack:",as_array"`
	PlayerIndex int
}
