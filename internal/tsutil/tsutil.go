package tsutil

import (
	"context"
	"sort"
	"strings"
	"time"

	"tailscale.com/client/tailscale"
)

// Node holds display information for a single Tailscale peer.
type Node struct {
	Name   string // display name (hostname)
	DNS    string // full MagicDNS name for connecting, e.g. "nautilus.magpie-cherimoya.ts.net"
	OS     string // "macOS", "Linux", "Windows", etc.
	Online bool
}

// Nodes queries the local Tailscale socket and returns the list of peers.
// Returns (nil, nil) if Tailscale is not running — callers treat nil as "not available".
func Nodes(ctx context.Context) ([]Node, error) {
	lc := &tailscale.LocalClient{}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	st, err := lc.Status(ctx)
	if err != nil {
		return nil, nil // degrade gracefully
	}

	var nodes []Node
	for _, peer := range st.Peer {
		name := peer.HostName
		dns := peer.DNSName
		// Trim trailing dot from DNS name if present.
		dns = strings.TrimSuffix(dns, ".")
		if dns == "" {
			dns = name
		}
		nodes = append(nodes, Node{
			Name:   name,
			DNS:    dns,
			OS:     capitalizeOS(peer.OS),
			Online: peer.Online,
		})
	}

	// Sort: online first, then alphabetical.
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Online != nodes[j].Online {
			return nodes[i].Online
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

func capitalizeOS(s string) string {
	switch strings.ToLower(s) {
	case "darwin", "macos":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		if s == "" {
			return "unknown"
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
}
