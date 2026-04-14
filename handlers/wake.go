package handlers

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// buildMagicPacket constructs a Wake-on-LAN magic packet for the given MAC address.
// A magic packet is exactly 102 bytes: 6×0xFF followed by the 6-byte MAC repeated 16 times.
// This layout is recognised by the NIC firmware regardless of the OS being asleep.
func buildMagicPacket(mac string) ([]byte, error) {
	// Accept both colon and hyphen separators
	parts := strings.Split(strings.ReplaceAll(mac, "-", ":"), ":")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid MAC address: %s", mac)
	}
	macBytes, err := hex.DecodeString(strings.Join(parts, ""))
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address: %w", err)
	}

	pkt := make([]byte, 102)
	for i := range 6 {
		pkt[i] = 0xFF
	}
	for i := range 16 {
		copy(pkt[(i+1)*6:], macBytes)
	}
	return pkt, nil
}

// subnetBroadcast returns the directed broadcast address for the first non-loopback
// IPv4 interface (e.g. 192.168.0.255 for a /24). This is more reliably forwarded
// by switches than the limited broadcast 255.255.255.255, which some equipment drops.
func subnetBroadcast() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "255.255.255.255"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			// Derive broadcast: host bits all set
			broadcast := make(net.IP, 4)
			for i := range 4 {
				broadcast[i] = ip[i] | ^ipNet.Mask[i]
			}
			return broadcast.String()
		}
	}
	return "255.255.255.255"
}

// wake handles POST /machines/{id}/wake.
// It looks up the machine's MAC address, builds a WoL magic packet, and
// broadcasts it as a UDP datagram to port 9 on the local network.
// No external library needed — net.Dial("udp", ...) is pure stdlib.
func (h *MachineHandler) wake(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := h.store.GetByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}

	pkt, err := buildMagicPacket(m.MAC)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	bcast := subnetBroadcast() + ":9"
	conn, err := net.Dial("udp", bcast)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not open UDP socket: "+err.Error())
		return
	}
	defer conn.Close()

	if _, err := conn.Write(pkt); err != nil {
		writeError(w, http.StatusInternalServerError, "UDP send failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"wake packet sent","broadcast":"` + bcast + `"}`)) //nolint:errcheck
}
