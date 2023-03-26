package subsystems

import "github.com/grandcat/zeroconf"

const (
	InstanceName = "Cyprus"
	Service      = "_cyprus._tcp"
	// "Default port" is all resolved in server.go
)

type NetworkDiscovery struct {
	server *zeroconf.Server
}

func NewNetworkDiscovery(port int) (*NetworkDiscovery, error) {
	zcServer, registerErr := zeroconf.Register(
		InstanceName,
		Service,
		"local.",
		port,
		// TODO: Instead of sending some random values, send some useful
		// 	information here.
		[]string{"txtv=0", "lo=1", "la=2"},
		nil,
	)
	if registerErr != nil {
		return nil, registerErr
	}
	return &NetworkDiscovery{zcServer}, nil
}

func (nt *NetworkDiscovery) Shutdown() {
	nt.server.Shutdown()
}
