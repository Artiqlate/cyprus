package subsystems

import (
	"fmt"

	"github.com/grandcat/zeroconf"
)

const (
	InstanceName = "Cyprus"
	Service      = "_cyprus._tcp"
	// "Default port" is all resolved in server.go
)

type NetworkDiscovery struct {
	server *zeroconf.Server
}

func NewNetworkDiscovery(port int, secure bool) (*NetworkDiscovery, error) {
	zcServer, registerErr := zeroconf.Register(
		InstanceName,
		Service,
		"local.",
		port,
		// If more information needs to be passed, add it here.
		[]string{fmt.Sprintf("secure=%t", secure)},
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
