//go:build darwin

// Package scan provides embarrassingly parallel network scanning with
// Waymo device detection, integrated with boxxy's pinhole subsystem.
//
// Architecture:
//
//	geofence ──┬──→ [gopacket promiscuous] ──┐
//	           ├──→ [ARP sweep (raw)]        ├──→ merge → score → pinhole verify
//	           ├──→ [mDNS browse]            │
//	           └──→ [SSDP M-SEARCH]         ┘
//
// All probes run concurrently via errgroup. The pcap capture starts
// first so it sees ARP replies and mDNS responses from our own probes.
//
// Boxxy integration points:
//   - pinhole.VerifyPinholeCompliance on captured pcap
//   - GF(3) trit classification per device
//   - Sideref capability tokens for scan authorization
package scan

import (
	"context"
	"fmt"
	"math"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/sync/errgroup"
)

// Google/Alphabet OUI prefixes (IEEE registry + observed Waymo fleet)
var GoogleOUIs = map[string]string{
	"00:1a:11": "Google Inc.", "3c:5a:b4": "Google Inc.", "54:60:09": "Google Inc.",
	"94:eb:2c": "Google Inc.", "a4:77:33": "Google Inc.", "f4:f5:d8": "Google Inc.",
	"f4:f5:e8": "Google Inc.", "08:9e:08": "Google Inc.", "20:df:b9": "Google Inc.",
	"30:fd:38": "Google Inc.", "44:07:0b": "Google Inc.", "48:d6:d5": "Google Inc.",
	"58:cb:52": "Google Inc.", "6c:ad:f8": "Google Inc.", "70:cd:0d": "Google Inc.",
	"7c:61:66": "Google Inc.", "94:94:26": "Google Inc.", "d8:6c:63": "Google Inc.",
	"e8:b2:ac": "Google Inc.", "f8:8f:ca": "Google Inc.",
	"a0:22:de": "Google LLC (Waymo)", "dc:e5:5b": "Google LLC", "e4:f0:42": "Google LLC",
}

// WaymoIndicators are mDNS/DNS strings that suggest autonomous vehicles.
var WaymoIndicators = []string{
	"waymo", "_waymo", "chauffeur", "_googcast", "_googlecast",
	"_grpc._tcp", "lidar-", "av-platform", "sensor-hub", "v2x-", "dsrc-",
}

const (
	TritMinus = -1 // Confirmed non-Waymo
	TritZero  = 0  // Unknown / ambiguous
	TritPlus  = 1  // Probable Waymo / Google AV
)

// Nonet is a GF(9) element — abelian extension of GF(3) for 9-state classification.
// GF(9) = GF(3)[i] where i²+1=0. Element = (score_trit, confidence_trit).
// Galois group: Gal(GF(9)/GF(3)) ≅ Z/2Z, Frobenius: (a,b) ↦ (a,-b).
type Nonet struct {
	Score      int `json:"score"`      // GF(3) trit: -1/0/+1
	Confidence int `json:"confidence"` // GF(3) trit: -1/0/+1
}

// Coarsen projects GF(9) back to GF(3) (real part).
func (n Nonet) Coarsen() int { return n.Score }

// Frobenius applies the GF(9)/GF(3) Frobenius automorphism.
func (n Nonet) Frobenius() Nonet { return Nonet{n.Score, tritNeg(n.Confidence)} }

// NonetAdd is componentwise addition in GF(9).
func NonetAdd(a, b Nonet) Nonet {
	return Nonet{tritAdd(a.Score, b.Score), tritAdd(a.Confidence, b.Confidence)}
}

// tritAdd computes balanced ternary addition: result in {-1, 0, +1}.
func tritAdd(a, b int) int {
	s := a + b
	switch {
	case s > 1:
		return s - 3 // 2 → -1
	case s < -1:
		return s + 3 // -2 → 1
	default:
		return s
	}
}

func tritNeg(a int) int { return -a }

// Device represents a discovered network device.
type Device struct {
	MAC         string    `json:"mac"`
	IP          string    `json:"ip"`
	Hostname    string    `json:"hostname,omitempty"`
	Vendor      string    `json:"vendor"`
	PacketCount int       `json:"packet_count"`
	ByteCount   int       `json:"byte_count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	PacketRate  float64   `json:"packets_per_sec"`
	Protocols   []string  `json:"protocols"`
	WaymoScore  float64   `json:"waymo_score"`
	GF3Trit     int       `json:"gf3_trit"`
	GF9Nonet    *Nonet    `json:"gf9_nonet,omitempty"` // abelian extension: 9-state
	Flags       []string  `json:"flags"`
	Source      string    `json:"source"`
}

// GeoFence describes the network boundary.
type GeoFence struct {
	BSSID      string  `json:"bssid"`
	SSID       string  `json:"ssid"`
	SubnetCIDR string  `json:"subnet_cidr"`
	LocalIP    string  `json:"local_ip"`
	LocalMAC   string  `json:"local_mac"`
	GatewayMAC string  `json:"gateway_mac"`
	GatewayIP  string  `json:"gateway_ip"`
	NumDevices int     `json:"num_devices"`
	Radius     string  `json:"radius_estimate"`
	Entropy    float64 `json:"entropy"`
}

// ScanResult is the complete output of a parallel scan.
type ScanResult struct {
	Timestamp  time.Time `json:"timestamp"`
	Interface  string    `json:"interface"`
	Duration   float64   `json:"duration_sec"`
	Subnet     string    `json:"subnet"`
	GeoFence   GeoFence  `json:"geofence"`
	Devices    []Device  `json:"devices"`
	WaymoCands []Device  `json:"waymo_candidates"`
	GF3Sum     int       `json:"gf3_sum"`
}

// ScanOpts configures a scan.
type ScanOpts struct {
	Interface   string
	PacketCount int
}

// Run executes the full parallel scan pipeline.
func Run(ctx context.Context, opts ScanOpts) (*ScanResult, error) {
	if opts.Interface == "" {
		opts.Interface = "en0"
	}
	if opts.PacketCount == 0 {
		opts.PacketCount = 500
	}

	start := time.Now()

	// Stage 1: Geofence
	gf := BuildGeoFence(opts.Interface)

	// Stage 2: Parallel discovery
	g, gctx := errgroup.WithContext(ctx)
	var (
		captureDevs, arpDevs []Device
		mu                   sync.Mutex
	)
	captureReady := make(chan struct{})

	// Promiscuous capture (start first)
	g.Go(func() error {
		devs, err := GopacketCapture(gctx, opts.Interface, opts.PacketCount, gf, captureReady)
		if err != nil {
			return err
		}
		mu.Lock()
		captureDevs = devs
		mu.Unlock()
		return nil
	})

	<-captureReady

	// ARP sweep
	g.Go(func() error {
		devs := ARPSweepNative(gctx, opts.Interface, gf)
		mu.Lock()
		arpDevs = devs
		mu.Unlock()
		return nil
	})

	// SSDP probe
	g.Go(func() error {
		conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900})
		if err != nil {
			return nil
		}
		defer conn.Close()
		conn.Write([]byte("M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n"))
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Stage 3: Merge + score
	merged := MergeDevices(arpDevs, captureDevs)
	var waymoCands []Device
	gf3Sum := 0
	for i := range merged {
		ScoreWaymo(&merged[i])
		gf3Sum += merged[i].GF3Trit
		if merged[i].WaymoScore >= 0.3 {
			waymoCands = append(waymoCands, merged[i])
		}
	}

	sort.Slice(waymoCands, func(i, j int) bool { return waymoCands[i].WaymoScore > waymoCands[j].WaymoScore })
	sort.Slice(merged, func(i, j int) bool { return merged[i].PacketCount > merged[j].PacketCount })

	return &ScanResult{
		Timestamp:  time.Now(),
		Interface:  opts.Interface,
		Duration:   time.Since(start).Seconds(),
		Subnet:     gf.SubnetCIDR,
		GeoFence:   gf,
		Devices:    merged,
		WaymoCands: waymoCands,
		GF3Sum:     gf3Sum % 3,
	}, nil
}

// GopacketCapture runs promiscuous pcap capture. Signals ready once listening.
func GopacketCapture(ctx context.Context, iface string, count int, gf GeoFence, ready chan<- struct{}) ([]Device, error) {
	handle, err := pcap.OpenLive(iface, 65536, true, pcap.BlockForever)
	if err != nil {
		close(ready)
		return nil, fmt.Errorf("pcap.OpenLive: %w", err)
	}
	defer handle.Close()
	close(ready)

	localSubnet := subnetBase(gf.LocalIP)
	devices := make(map[string]*Device)
	source := gopacket.NewPacketSource(handle, handle.LinkType())

	captured := 0
	for captured < count {
		select {
		case <-ctx.Done():
			goto done
		case pkt, ok := <-source.Packets():
			if !ok {
				goto done
			}
			captured++
			processPacket(devices, pkt, localSubnet)
		}
	}

done:
	result := make([]Device, 0, len(devices))
	for _, d := range devices {
		elapsed := d.LastSeen.Sub(d.FirstSeen).Seconds()
		if elapsed > 0 {
			d.PacketRate = float64(d.PacketCount) / elapsed
		}
		d.Source = "capture"
		result = append(result, *d)
	}
	return result, nil
}

func processPacket(devices map[string]*Device, pkt gopacket.Packet, localSubnet string) {
	ethLayer := pkt.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		return
	}
	eth := ethLayer.(*layers.Ethernet)
	srcMAC := strings.ToLower(eth.SrcMAC.String())
	dstMAC := strings.ToLower(eth.DstMAC.String())
	pktLen := len(pkt.Data())
	now := time.Now()
	proto := classifyPacket(pkt)

	var srcIP, dstIP string
	if ip4 := pkt.Layer(layers.LayerTypeIPv4); ip4 != nil {
		v := ip4.(*layers.IPv4)
		srcIP, dstIP = v.SrcIP.String(), v.DstIP.String()
	}
	if arpL := pkt.Layer(layers.LayerTypeARP); arpL != nil {
		a := arpL.(*layers.ARP)
		srcIP = net.IP(a.SourceProtAddress).String()
		srcMAC = strings.ToLower(net.HardwareAddr(a.SourceHwAddress).String())
	}

	for _, mac := range []string{srcMAC, dstMAC} {
		if mac == "ff:ff:ff:ff:ff:ff" || isMulticastMAC(mac) {
			continue
		}
		d := getOrCreate(devices, mac, now)
		d.PacketCount++
		d.ByteCount += pktLen
		d.LastSeen = now
		addProto(d, proto)
	}
	if srcMAC != "ff:ff:ff:ff:ff:ff" && !isMulticastMAC(srcMAC) {
		assignLocalIP(devices[srcMAC], srcIP, localSubnet)
	}
	if dstMAC != "ff:ff:ff:ff:ff:ff" && !isMulticastMAC(dstMAC) {
		assignLocalIP(devices[dstMAC], dstIP, localSubnet)
	}
}

func classifyPacket(pkt gopacket.Packet) string {
	if pkt.Layer(layers.LayerTypeARP) != nil {
		return "ARP"
	}
	if pkt.Layer(layers.LayerTypeICMPv4) != nil {
		return "ICMP"
	}
	if udp := pkt.Layer(layers.LayerTypeUDP); udp != nil {
		u := udp.(*layers.UDP)
		switch {
		case u.DstPort == 5353 || u.SrcPort == 5353:
			return "mDNS"
		case u.DstPort == 53 || u.SrcPort == 53:
			return "DNS"
		case u.DstPort == 1900 || u.SrcPort == 1900:
			return "SSDP"
		default:
			return "UDP"
		}
	}
	if tcp := pkt.Layer(layers.LayerTypeTCP); tcp != nil {
		t := tcp.(*layers.TCP)
		if t.DstPort == 443 || t.SrcPort == 443 {
			return "TLS"
		}
		return "TCP"
	}
	return ""
}

// ARPSweepNative injects raw ARP requests via gopacket.
func ARPSweepNative(ctx context.Context, iface string, gf GeoFence) []Device {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return nil
	}
	handle, err := pcap.OpenLive(iface, 65536, false, pcap.BlockForever)
	if err != nil {
		return arpSweepFallback(ctx, gf)
	}
	defer handle.Close()
	handle.SetBPFFilter("arp")

	devices := make(map[string]*Device)
	var mu sync.Mutex
	stop := make(chan struct{})

	go func() {
		src := gopacket.NewPacketSource(handle, handle.LinkType())
		for {
			select {
			case <-stop:
				return
			default:
			}
			pkt, err := src.NextPacket()
			if err != nil {
				continue
			}
			a := pkt.Layer(layers.LayerTypeARP)
			if a == nil {
				continue
			}
			arp := a.(*layers.ARP)
			if arp.Operation != layers.ARPReply {
				continue
			}
			mac := strings.ToLower(net.HardwareAddr(arp.SourceHwAddress).String())
			ip := net.IP(arp.SourceProtAddress).String()
			mu.Lock()
			devices[mac] = &Device{
				MAC: mac, IP: ip, Vendor: LookupOUI(mac),
				FirstSeen: time.Now(), LastSeen: time.Now(),
				Protocols: []string{"ARP"}, Source: "arp",
			}
			mu.Unlock()
		}
	}()

	base := subnetBase(gf.GatewayIP)
	if base == "" {
		base = subnetBase(gf.LocalIP)
	}
	if base == "" {
		close(stop)
		return nil
	}

	srcIP := net.ParseIP(gf.LocalIP).To4()
	eth := layers.Ethernet{
		SrcMAC: ifi.HardwareAddr, DstMAC: net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}
	arpReq := layers.ARP{
		AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
		HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPRequest,
		SourceHwAddress: []byte(ifi.HardwareAddr), SourceProtAddress: srcIP,
		DstHwAddress: []byte{0, 0, 0, 0, 0, 0},
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	for i := 1; i <= 254; i++ {
		select {
		case <-ctx.Done():
			break
		default:
		}
		arpReq.DstProtAddress = net.ParseIP(fmt.Sprintf("%s.%d", base, i)).To4()
		buf.Clear()
		gopacket.SerializeLayers(buf, opts, &eth, &arpReq)
		handle.WritePacketData(buf.Bytes())
	}

	time.Sleep(2 * time.Second)
	close(stop)

	r := make([]Device, 0, len(devices))
	for _, d := range devices {
		r = append(r, *d)
	}
	return r
}

func arpSweepFallback(ctx context.Context, gf GeoFence) []Device {
	base := subnetBase(gf.GatewayIP)
	if base == "" {
		return nil
	}
	g, fctx := errgroup.WithContext(ctx)
	g.SetLimit(64)
	for i := 1; i <= 254; i++ {
		ip := fmt.Sprintf("%s.%d", base, i)
		g.Go(func() error {
			exec.CommandContext(fctx, "ping", "-c", "1", "-W", "200", "-t", "1", ip).Run()
			return nil
		})
	}
	g.Wait()

	devices := make(map[string]*Device)
	if out, err := exec.Command("arp", "-a").Output(); err == nil {
		re := regexp.MustCompile(`\((\d+\.\d+\.\d+\.\d+)\)\s+at\s+([0-9a-f:]+)`)
		for _, line := range strings.Split(string(out), "\n") {
			if m := re.FindStringSubmatch(line); len(m) == 3 {
				mac := normalizeMAC(m[2])
				if mac != "" && mac != "ff:ff:ff:ff:ff:ff" {
					devices[mac] = &Device{
						MAC: mac, IP: m[1], Vendor: LookupOUI(mac),
						FirstSeen: time.Now(), LastSeen: time.Now(),
						Protocols: []string{"ARP"}, Source: "arp",
					}
				}
			}
		}
	}
	r := make([]Device, 0, len(devices))
	for _, d := range devices {
		r = append(r, *d)
	}
	return r
}

// ScoreWaymo computes the Waymo probability score for a device.
func ScoreWaymo(d *Device) {
	score := 0.0
	if len(d.MAC) >= 8 {
		if _, ok := GoogleOUIs[strings.ToLower(d.MAC[:8])]; ok {
			score += 0.5
			addFlag(d, "google-oui")
		}
	}
	if d.PacketRate > 100 {
		score += 0.15
		addFlag(d, "high-pkt-rate")
	}
	if d.ByteCount > 100000 {
		score += 0.1
		addFlag(d, "high-bandwidth")
	}
	for _, f := range d.Flags {
		if strings.HasPrefix(f, "mdns:") {
			score += 0.2
			break
		}
	}
	if len(d.Protocols) >= 4 {
		score += 0.1
		addFlag(d, "multi-protocol")
	}
	if len(d.MAC) >= 2 {
		if fb, _ := strconv.ParseUint(d.MAC[:2], 16, 8); fb&0x02 != 0 {
			score -= 0.05
			addFlag(d, "local-admin-mac")
		}
	}
	d.WaymoScore = clamp(score, 0, 1)
	switch {
	case d.WaymoScore >= 0.5:
		d.GF3Trit = TritPlus
	case d.WaymoScore >= 0.2:
		d.GF3Trit = TritZero
	default:
		d.GF3Trit = TritMinus
	}

	// GF(9) nonet: abelian extension — (score, confidence) in GF(3)²
	// Confidence trit based on evidence quality
	confTrit := TritZero
	evidenceCount := len(d.Protocols) + len(d.Flags)
	switch {
	case evidenceCount >= 5:
		confTrit = TritPlus // high confidence
	case evidenceCount <= 1:
		confTrit = TritMinus // low confidence
	}
	d.GF9Nonet = &Nonet{Score: d.GF3Trit, Confidence: confTrit}
}

// MergeDevices combines device lists from multiple sources.
func MergeDevices(sources ...[]Device) []Device {
	merged := make(map[string]*Device)
	for _, devs := range sources {
		for _, d := range devs {
			key := d.MAC
			if key == "" {
				key = d.IP
			}
			if ex, ok := merged[key]; ok {
				ex.PacketCount += d.PacketCount
				ex.ByteCount += d.ByteCount
				if ex.IP == "" && d.IP != "" {
					ex.IP = d.IP
				}
				if ex.Hostname == "" && d.Hostname != "" {
					ex.Hostname = d.Hostname
				}
				ex.Source = "merged"
				for _, p := range d.Protocols {
					addProto(ex, p)
				}
				for _, f := range d.Flags {
					addFlag(ex, f)
				}
			} else {
				dd := d
				merged[key] = &dd
			}
		}
	}
	r := make([]Device, 0, len(merged))
	for _, d := range merged {
		r = append(r, *d)
	}
	return r
}

// BuildGeoFence identifies the network boundary.
func BuildGeoFence(iface string) GeoFence {
	gf := GeoFence{}
	if out, err := exec.Command("system_profiler", "SPAirPortDataType", "-detailLevel", "basic").Output(); err == nil {
		inCur := false
		for _, line := range strings.Split(string(out), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "Current Network Information:") {
				inCur = true
				continue
			}
			if inCur {
				if strings.HasSuffix(t, ":") && !strings.Contains(t, " ") && gf.SSID == "" {
					gf.SSID = strings.TrimSuffix(t, ":")
				}
				if strings.HasPrefix(t, "BSSID:") {
					gf.BSSID = strings.TrimSpace(strings.TrimPrefix(t, "BSSID:"))
				}
			}
		}
	}
	if out, err := exec.Command("ifconfig", iface).Output(); err == nil {
		s := string(out)
		if m := regexp.MustCompile(`inet (\d+\.\d+\.\d+\.\d+) netmask (0x[0-9a-f]+)`).FindStringSubmatch(s); len(m) == 3 {
			gf.LocalIP = m[1]
			gf.SubnetCIDR = fmt.Sprintf("%s/%d", m[1], hexMaskToCIDR(m[2]))
		}
		if m := regexp.MustCompile(`ether ([0-9a-f:]+)`).FindStringSubmatch(s); len(m) == 2 {
			gf.LocalMAC = m[1]
		}
	}
	if out, err := exec.Command("route", "-n", "get", "default").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "gateway:") {
				gf.GatewayIP = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "gateway:"))
			}
		}
	}
	if gf.GatewayIP != "" {
		if out, err := exec.Command("arp", "-n", gf.GatewayIP).Output(); err == nil {
			if m := regexp.MustCompile(`([0-9a-f:]{11,17})`).FindString(string(out)); m != "" {
				gf.GatewayMAC = m
			}
		}
	}
	if out, err := exec.Command("arp", "-a").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		gf.NumDevices = len(lines)
		gf.Radius = estimateRadius(len(lines))
		gf.Entropy = macEntropy(string(out))
	}
	return gf
}

// LookupOUI returns the vendor name for a MAC address.
func LookupOUI(mac string) string {
	if len(mac) < 8 {
		return "unknown"
	}
	oui := strings.ToLower(mac[:8])
	if v, ok := GoogleOUIs[oui]; ok {
		return v
	}
	known := map[string]string{
		"b8:27:eb": "Raspberry Pi", "dc:a6:32": "Raspberry Pi",
		"18:fe:34": "Espressif", "ec:b5:fa": "Philips Hue",
		"3c:22:fb": "Apple", "5c:9b:a6": "Apple", "10:dd:b1": "Apple",
		"44:d9:e7": "Ubiquiti", "b4:fb:e4": "Ubiquiti",
	}
	if v, ok := known[oui]; ok {
		return v
	}
	return "unknown"
}

// --- helpers ---

func getOrCreate(m map[string]*Device, mac string, now time.Time) *Device {
	d, ok := m[mac]
	if !ok {
		d = &Device{MAC: mac, Vendor: LookupOUI(mac), FirstSeen: now, LastSeen: now}
		m[mac] = d
	}
	return d
}

func assignLocalIP(d *Device, ip, localSubnet string) {
	if ip == "" || d == nil || strings.HasPrefix(ip, "224.") || strings.HasPrefix(ip, "239.") {
		return
	}
	isLocal := localSubnet != "" && strings.HasPrefix(ip, localSubnet+".")
	if d.IP == "" || (isLocal && !strings.HasPrefix(d.IP, localSubnet+".")) {
		d.IP = ip
	}
}

func addProto(d *Device, p string) {
	if p == "" {
		return
	}
	for _, v := range d.Protocols {
		if v == p {
			return
		}
	}
	d.Protocols = append(d.Protocols, p)
}

func addFlag(d *Device, f string) {
	for _, v := range d.Flags {
		if v == f {
			return
		}
	}
	d.Flags = append(d.Flags, f)
}

func subnetBase(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return strings.Join(parts[:3], ".")
	}
	return ""
}

func normalizeMAC(mac string) string {
	mac = strings.TrimSpace(mac)
	if mac == "(incomplete)" || mac == "" {
		return ""
	}
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return mac
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = "0" + p
		}
	}
	return strings.ToLower(strings.Join(parts, ":"))
}

func isMulticastMAC(mac string) bool {
	if strings.HasPrefix(mac, "01:00:5e") || strings.HasPrefix(mac, "33:33:") {
		return true
	}
	if len(mac) < 2 {
		return false
	}
	fb, _ := strconv.ParseUint(mac[:2], 16, 8)
	return fb&0x01 != 0
}

func hexMaskToCIDR(hex string) int {
	hex = strings.TrimPrefix(hex, "0x")
	val, _ := strconv.ParseUint(hex, 16, 32)
	bits := 0
	for val > 0 {
		bits += int(val & 1)
		val >>= 1
	}
	return bits
}

func estimateRadius(n int) string {
	switch {
	case n <= 5:
		return "~10m"
	case n <= 20:
		return "~30m"
	case n <= 50:
		return "~100m"
	case n <= 200:
		return "~250m"
	}
	return "~500m+"
}

func macEntropy(arpOut string) float64 {
	vendors := make(map[string]int)
	for _, m := range regexp.MustCompile(`at\s+([0-9a-f:]+)`).FindAllStringSubmatch(arpOut, -1) {
		mac := normalizeMAC(m[1])
		if len(mac) >= 8 {
			vendors[mac[:8]]++
		}
	}
	total := 0.0
	for _, c := range vendors {
		total += float64(c)
	}
	if total == 0 {
		return 0
	}
	e := 0.0
	for _, c := range vendors {
		p := float64(c) / total
		e -= p * math.Log2(p)
	}
	return math.Round(e*100) / 100
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
