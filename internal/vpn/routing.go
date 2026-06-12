package vpn

import (
	_ "embed"
	"encoding/binary"
	"math/bits"
	"net"
	"sort"
	"strings"
	"sync"
)

// RoutingMode controls which traffic goes through the tunnel.
type RoutingMode string

const (
	// RoutingFull sends all traffic through the VPN.
	RoutingFull RoutingMode = "full"
	// RoutingSplitRU sends everything through the VPN except Russian and private
	// networks, which go direct — so Russian banks/gov services keep working.
	RoutingSplitRU RoutingMode = "split_ru"
)

// Valid reports whether m is a known routing mode.
func (m RoutingMode) Valid() bool { return m == RoutingFull || m == RoutingSplitRU }

//go:embed data/ru_cidrs.txt
var ruCIDRs string

// privateCIDRs are always routed directly (LAN, loopback, reserved, multicast).
var privateCIDRs = []string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.168.0.0/16", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
}

type ipRange struct{ start, end uint32 }

var (
	splitOnce    sync.Once
	splitAllowed string
)

// fullAllowedIPs routes everything through the tunnel.
const fullAllowedIPs = "0.0.0.0/0, ::/0"

// allowedIPsFor returns the WireGuard AllowedIPs value for a routing mode.
func allowedIPsFor(mode RoutingMode) string {
	if mode != RoutingSplitRU {
		return fullAllowedIPs
	}
	splitOnce.Do(func() {
		cidrs := computeComplement(excludedRanges())
		// IPv6 is left direct in split mode (nodes are IPv4-only here), avoiding
		// breakage; IPv4 covers Russian banking/gov resources.
		splitAllowed = strings.Join(cidrs, ", ")
	})
	return splitAllowed
}

// excludedRanges parses the Russian + private CIDRs into merged numeric ranges.
func excludedRanges() []ipRange {
	var ranges []ipRange
	add := func(cidr string) {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			return
		}
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return
		}
		ip4 := n.IP.To4()
		if ip4 == nil {
			return
		}
		ones, _ := n.Mask.Size()
		start := binary.BigEndian.Uint32(ip4)
		size := uint32(1) << uint(32-ones)
		ranges = append(ranges, ipRange{start: start, end: start + size - 1})
	}
	for _, c := range strings.Split(ruCIDRs, "\n") {
		add(c)
	}
	for _, c := range privateCIDRs {
		add(c)
	}
	return mergeRanges(ranges)
}

func mergeRanges(in []ipRange) []ipRange {
	if len(in) == 0 {
		return in
	}
	sort.Slice(in, func(i, j int) bool { return in[i].start < in[j].start })
	merged := []ipRange{in[0]}
	for _, r := range in[1:] {
		last := &merged[len(merged)-1]
		if r.start <= last.end+1 && last.end != ^uint32(0) {
			if r.end > last.end {
				last.end = r.end
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// computeComplement returns the CIDRs covering all IPv4 addresses NOT in the
// excluded (merged, sorted) ranges.
func computeComplement(excluded []ipRange) []string {
	var out []string
	var cursor uint32 // next address not yet covered
	started := false

	for _, r := range excluded {
		if r.start > cursor || (!started && r.start == 0) {
			if r.start > cursor {
				out = append(out, rangeToCIDRs(cursor, r.start-1)...)
			}
		}
		if r.end == ^uint32(0) {
			return out
		}
		cursor = r.end + 1
		started = true
	}
	out = append(out, rangeToCIDRs(cursor, ^uint32(0))...)
	return out
}

// rangeToCIDRs converts an inclusive [start,end] IPv4 range into aligned CIDRs.
func rangeToCIDRs(start, end uint32) []string {
	var out []string
	for start <= end {
		// Largest aligned block starting at start.
		maxSize := uint32(32)
		if start != 0 {
			maxSize = uint32(bits.TrailingZeros32(start))
		}
		// Largest block that fits in the remaining range.
		remaining := uint64(end) - uint64(start) + 1
		fitBits := uint32(63 - bits.LeadingZeros64(remaining))
		if fitBits < maxSize {
			maxSize = fitBits
		}
		prefix := 32 - maxSize
		out = append(out, ipString(start)+"/"+itoa(int(prefix)))
		blockSize := uint64(1) << maxSize
		next := uint64(start) + blockSize
		if next > uint64(^uint32(0)) {
			break
		}
		start = uint32(next)
	}
	return out
}

func ipString(v uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, v)
	return ip.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [3]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
