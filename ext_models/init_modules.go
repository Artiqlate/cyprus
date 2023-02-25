package ext_models

// TODO: Move this to ganymede
type InitModules struct {
	_msgpack       struct{} `msgpack:",as_array"`
	EnabledModules []string `msgpack:"enabledModules"`
}

func NewInitModules(enabledModules []string) *InitModules {
	return &InitModules{
		EnabledModules: enabledModules,
	}
}
