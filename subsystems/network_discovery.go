package subsystems

import "github.com/grandcat/zeroconf"

const (
	InstanceName = "Cyprus"
	Service      = "_cyprus._tcp"
	Port         = 8000
)

type NetworkDiscovery struct {
	server *zeroconf.Server
}

func NewNetworkDiscovery() (*NetworkDiscovery, error) {
	zcServer, registerErr := zeroconf.Register(
		InstanceName,
		Service,
		"local.",
		Port,
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
