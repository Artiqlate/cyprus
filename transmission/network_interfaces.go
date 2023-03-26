package transmission

import (
	"net"
)

func getAvailableIPAddresses() ([]net.IP, error) {
	var availableIpAddresses []net.IP
	ifaces, ifacesErr := net.Interfaces()
	if ifacesErr != nil {
		return []net.IP{}, ifacesErr
	}
	for _, iface := range ifaces {
		// Ignore all loop-back and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Get addresses associated w/ the interface
		addresses, addrErr := iface.Addrs()
		if addrErr != nil {
			return []net.IP{}, addrErr
		}

		// Iterate over the addresses
		for _, addr := range addresses {
			// Check if address is an IP Address
			ip, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			// Check if it's not an IP Address
			if ip.IP.To4() == nil {
				continue
			}
			// Add to the list of available IP Addresses
			availableIpAddresses = append(availableIpAddresses, ip.IP)
		}
	}
	// log.Printf("Available IP Addresses: %s", availableIpAddresses)
	return availableIpAddresses, nil
}
