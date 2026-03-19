//go:build darwin

package scan

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// Task represents a unit of scan work.
type Task struct {
	Kind    TaskKind
	IP      string // for ARP tasks
	Subnet  string // for range tasks
	StartIP int    // for range subdivision
	EndIP   int    // for range subdivision
	Packet  gopacket.Packet // for packet processing tasks
}

type TaskKind int

const (
	TaskARP    TaskKind = iota // Send ARP request to single IP
	TaskRange                  // ARP sweep a range (stealable/splittable)
	TaskPacket                 // Process a captured packet
	TaskMDNS                   // mDNS query for a service
	TaskSSDP                   // SSDP M-SEARCH
)

// Deque is a work-stealing double-ended queue.
// Owner pushes/pops from the bottom (LIFO for locality).
// Thieves steal from the top (FIFO for fairness).
// Based on the Chase-Lev deque used in Go's runtime.
type Deque struct {
	tasks  []Task
	top    atomic.Int64 // thieves steal from here
	bottom atomic.Int64 // owner pushes/pops here
	mu     sync.Mutex   // only for grow operations
}

func NewDeque(cap int) *Deque {
	return &Deque{tasks: make([]Task, cap)}
}

// Push adds a task to the bottom (owner only, no lock needed for single-owner).
func (d *Deque) Push(t Task) {
	b := d.bottom.Load()
	tp := d.top.Load()
	size := b - tp
	if size >= int64(len(d.tasks)) {
		d.grow()
	}
	d.tasks[b%int64(len(d.tasks))] = t
	d.bottom.Store(b + 1)
}

// Pop takes a task from the bottom (owner only).
func (d *Deque) Pop() (Task, bool) {
	b := d.bottom.Load() - 1
	d.bottom.Store(b)
	t := d.top.Load()

	if t <= b {
		// Non-empty, take the task
		return d.tasks[b%int64(len(d.tasks))], true
	}

	if t == b+1 {
		// Empty
		d.bottom.Store(t)
		return Task{}, false
	}

	// One element, race with stealers
	task := d.tasks[b%int64(len(d.tasks))]
	if d.top.CompareAndSwap(t, t+1) {
		d.bottom.Store(t + 1)
		return task, true
	}
	d.bottom.Store(t + 1)
	return Task{}, false
}

// Steal takes a task from the top (thieves, lock-free CAS).
func (d *Deque) Steal() (Task, bool) {
	t := d.top.Load()
	b := d.bottom.Load()
	if t >= b {
		return Task{}, false // empty
	}
	task := d.tasks[t%int64(len(d.tasks))]
	if d.top.CompareAndSwap(t, t+1) {
		return task, true
	}
	return Task{}, false // lost race, try again
}

// StealHalf takes up to half the tasks (bulk steal for efficiency).
func (d *Deque) StealHalf() []Task {
	t := d.top.Load()
	b := d.bottom.Load()
	size := b - t
	if size <= 0 {
		return nil
	}
	n := size / 2
	if n == 0 {
		n = 1
	}
	var stolen []Task
	for i := int64(0); i < n; i++ {
		ct := d.top.Load()
		if ct >= d.bottom.Load() {
			break
		}
		task := d.tasks[ct%int64(len(d.tasks))]
		if d.top.CompareAndSwap(ct, ct+1) {
			stolen = append(stolen, task)
		} else {
			break // contention, bail
		}
	}
	return stolen
}

// Size returns approximate queue size.
func (d *Deque) Size() int {
	return int(d.bottom.Load() - d.top.Load())
}

func (d *Deque) grow() {
	d.mu.Lock()
	defer d.mu.Unlock()
	newCap := len(d.tasks) * 2
	newTasks := make([]Task, newCap)
	t := d.top.Load()
	b := d.bottom.Load()
	for i := t; i < b; i++ {
		newTasks[i%int64(newCap)] = d.tasks[i%int64(len(d.tasks))]
	}
	d.tasks = newTasks
}

// Worker is a scan worker with its own deque.
type Worker struct {
	ID       int
	Deque    *Deque
	Devices  map[string]*Device
	Stats    WorkerStats
	handle   *pcap.Handle // shared pcap handle for ARP injection
	localMAC net.HardwareAddr
	localIP  net.IP
}

type WorkerStats struct {
	TasksExecuted int64
	TasksStolen   int64
	ARPsSent      int64
	PacketsProc   int64
	StealAttempts int64
	StealSuccess  int64
}

// Scheduler coordinates work-stealing across workers.
type Scheduler struct {
	Workers     []*Worker
	GlobalQueue chan Task // checked 1/61 iterations (like Go runtime)
	GeoFence    GeoFence
	iface       string
	handle      *pcap.Handle
	localMAC    net.HardwareAddr
	localIP     net.IP

	feeding  atomic.Bool // true while DistributePackets is still running
	done     atomic.Bool
	resultMu sync.Mutex
	results  map[string]*Device
}

// NewScheduler creates a work-stealing scanner with N workers.
func NewScheduler(iface string, gf GeoFence, numWorkers int) (*Scheduler, error) {
	if numWorkers == 0 {
		numWorkers = runtime.NumCPU()
	}

	// Open shared pcap handle for ARP injection
	handle, err := pcap.OpenLive(iface, 65536, false, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("pcap open for ARP: %w", err)
	}

	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("interface %s: %w", iface, err)
	}

	localIP := net.ParseIP(gf.LocalIP).To4()

	s := &Scheduler{
		Workers:     make([]*Worker, numWorkers),
		GlobalQueue: make(chan Task, 1024),
		GeoFence:    gf,
		iface:       iface,
		handle:      handle,
		localMAC:    ifi.HardwareAddr,
		localIP:     localIP,
		results:     make(map[string]*Device),
	}

	for i := 0; i < numWorkers; i++ {
		s.Workers[i] = &Worker{
			ID:       i,
			Deque:    NewDeque(256),
			Devices:  make(map[string]*Device),
			handle:   handle,
			localMAC: ifi.HardwareAddr,
			localIP:  localIP,
		}
	}

	return s, nil
}

// Close releases resources.
func (s *Scheduler) Close() {
	if s.handle != nil {
		s.handle.Close()
	}
}

// DistributeARPSweep splits 1-254 across workers as stealable range tasks.
func (s *Scheduler) DistributeARPSweep() {
	n := len(s.Workers)
	base := subnetBase(s.GeoFence.GatewayIP)
	if base == "" {
		base = subnetBase(s.GeoFence.LocalIP)
	}
	if base == "" {
		return
	}

	// Split 254 IPs across workers
	perWorker := 254 / n
	for i := 0; i < n; i++ {
		start := i*perWorker + 1
		end := start + perWorker - 1
		if i == n-1 {
			end = 254 // last worker gets remainder
		}
		s.Workers[i].Deque.Push(Task{
			Kind:    TaskRange,
			Subnet:  base,
			StartIP: start,
			EndIP:   end,
		})
	}
}

// DistributePackets feeds captured packets to workers round-robin.
// Sets s.feeding=true while active so workers don't exit prematurely.
func (s *Scheduler) DistributePackets(ctx context.Context, packetCount int) error {
	s.feeding.Store(true)
	defer s.feeding.Store(false)

	handle, err := pcap.OpenLive(s.iface, 65536, true, pcap.BlockForever)
	if err != nil {
		return err
	}
	defer handle.Close()

	source := gopacket.NewPacketSource(handle, handle.LinkType())
	captured := 0
	for captured < packetCount {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case pkt, ok := <-source.Packets():
			if !ok {
				return nil
			}
			w := s.Workers[captured%len(s.Workers)]
			w.Deque.Push(Task{Kind: TaskPacket, Packet: pkt})
			captured++
		}
	}
	return nil
}

// AddGlobalTask adds a task to the global queue (checked infrequently).
func (s *Scheduler) AddGlobalTask(t Task) {
	select {
	case s.GlobalQueue <- t:
	default:
		// Global queue full, push to random worker
		w := s.Workers[rand.IntN(len(s.Workers))]
		w.Deque.Push(t)
	}
}

// Run starts all workers and a dedicated ARP reply listener, then waits.
func (s *Scheduler) Run(ctx context.Context) *ScanResult {
	start := time.Now()
	var wg sync.WaitGroup

	// Dedicated ARP reply listener (separate BPF-filtered handle)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.listenARPReplies(ctx)
	}()

	for _, w := range s.Workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			s.workerLoop(ctx, w)
		}(w)
	}

	wg.Wait()
	s.done.Store(true)

	// Merge all worker device maps
	s.resultMu.Lock()
	for _, w := range s.Workers {
		for mac, d := range w.Devices {
			if existing, ok := s.results[mac]; ok {
				existing.PacketCount += d.PacketCount
				existing.ByteCount += d.ByteCount
				if existing.IP == "" && d.IP != "" {
					existing.IP = d.IP
				}
				existing.Source = "merged"
				for _, p := range d.Protocols {
					addProto(existing, p)
				}
				for _, f := range d.Flags {
					addFlag(existing, f)
				}
			} else {
				dd := *d
				s.results[mac] = &dd
			}
		}
	}
	s.resultMu.Unlock()

	// Score and build result
	var devices []Device
	var waymoCands []Device
	gf3Sum := 0

	s.resultMu.Lock()
	for _, d := range s.results {
		elapsed := d.LastSeen.Sub(d.FirstSeen).Seconds()
		if elapsed > 0 {
			d.PacketRate = float64(d.PacketCount) / elapsed
		}
		ScoreWaymo(d)
		gf3Sum += d.GF3Trit
		devices = append(devices, *d)
		if d.WaymoScore >= 0.3 {
			waymoCands = append(waymoCands, *d)
		}
	}
	s.resultMu.Unlock()

	return &ScanResult{
		Timestamp:  time.Now(),
		Interface:  s.iface,
		Duration:   time.Since(start).Seconds(),
		Subnet:     s.GeoFence.SubnetCIDR,
		GeoFence:   s.GeoFence,
		Devices:    devices,
		WaymoCands: waymoCands,
		GF3Sum:     gf3Sum % 3,
	}
}

// listenARPReplies runs a dedicated BPF-filtered handle that captures ARP replies
// from our sweep and adds them directly to the result map. This is separate from
// the promiscuous capture so ARP replies don't compete with regular traffic.
func (s *Scheduler) listenARPReplies(ctx context.Context) {
	// Use short pcap timeout so NextPacket doesn't block forever
	handle, err := pcap.OpenLive(s.iface, 65536, false, 100*time.Millisecond)
	if err != nil {
		return
	}
	defer handle.Close()
	handle.SetBPFFilter("arp")

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	src.DecodeOptions.Lazy = true
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case pkt, ok := <-src.Packets():
			if !ok {
				return
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
			now := time.Now()

			s.resultMu.Lock()
			if _, ok := s.results[mac]; !ok {
				s.results[mac] = &Device{
					MAC: mac, IP: ip, Vendor: LookupOUI(mac),
					FirstSeen: now, LastSeen: now,
					Protocols: []string{"ARP"}, Source: "arp-ws",
				}
			}
			s.resultMu.Unlock()
		}
	}
}

// workerLoop is the per-worker scheduling loop (mirrors Go runtime GMP).
func (s *Scheduler) workerLoop(ctx context.Context, w *Worker) {
	iteration := 0
	idleSpins := 0
	maxIdleSpins := 32 // spin before parking

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		iteration++
		var task Task
		found := false

		// 1/61 check global queue (same ratio as Go runtime)
		if iteration%61 == 0 {
			select {
			case task = <-s.GlobalQueue:
				found = true
			default:
			}
		}

		// Check own deque
		if !found {
			task, found = w.Deque.Pop()
		}

		// Steal from random peer (steal half for amortization)
		if !found {
			atomic.AddInt64(&w.Stats.StealAttempts, 1)
			victim := s.Workers[rand.IntN(len(s.Workers))]
			if victim.ID != w.ID {
				stolen := victim.Deque.StealHalf()
				if len(stolen) > 0 {
					atomic.AddInt64(&w.Stats.StealSuccess, 1)
					atomic.AddInt64(&w.Stats.TasksStolen, int64(len(stolen)))
					// Execute first, push rest to own deque
					task = stolen[0]
					found = true
					for _, t := range stolen[1:] {
						w.Deque.Push(t)
					}
				}
			}
		}

		// Recheck global queue
		if !found {
			select {
			case task = <-s.GlobalQueue:
				found = true
			default:
			}
		}

		if !found {
			idleSpins++
			if idleSpins > maxIdleSpins && !s.feeding.Load() {
				// All queues empty + spun enough + no more packets coming → done
				return
			}
			if idleSpins > maxIdleSpins {
				// Still feeding — sleep briefly instead of busy-spinning
				time.Sleep(time.Millisecond)
				idleSpins = maxIdleSpins / 2 // partial reset to stay responsive
			}
			runtime.Gosched()
			continue
		}

		// Reset idle counter on work found
		idleSpins = 0
		atomic.AddInt64(&w.Stats.TasksExecuted, 1)

		// Execute the task
		s.executeTask(ctx, w, task)
	}
}

func (s *Scheduler) executeTask(ctx context.Context, w *Worker, t Task) {
	switch t.Kind {
	case TaskARP:
		s.executeARP(w, t.IP)
		atomic.AddInt64(&w.Stats.ARPsSent, 1)

	case TaskRange:
		// Split range into individual ARPs — these can be stolen
		for i := t.StartIP; i <= t.EndIP; i++ {
			ip := fmt.Sprintf("%s.%d", t.Subnet, i)
			// If range is large enough, push as sub-tasks (stealable)
			if t.EndIP-t.StartIP > 16 {
				w.Deque.Push(Task{Kind: TaskARP, IP: ip})
			} else {
				// Small range, just do it directly
				s.executeARP(w, ip)
				atomic.AddInt64(&w.Stats.ARPsSent, 1)
			}
		}

	case TaskPacket:
		s.executePacketProcess(w, t.Packet)
		atomic.AddInt64(&w.Stats.PacketsProc, 1)

	case TaskMDNS:
		// mDNS query handled at global level
	case TaskSSDP:
		// SSDP handled at global level
	}
}

func (s *Scheduler) executeARP(w *Worker, ip string) {
	targetIP := net.ParseIP(ip).To4()
	if targetIP == nil {
		return
	}

	eth := layers.Ethernet{
		SrcMAC:       s.localMAC,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
		HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPRequest,
		SourceHwAddress: []byte(s.localMAC), SourceProtAddress: s.localIP,
		DstHwAddress: []byte{0, 0, 0, 0, 0, 0}, DstProtAddress: targetIP,
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	gopacket.SerializeLayers(buf, opts, &eth, &arp)
	s.handle.WritePacketData(buf.Bytes())
}

func (s *Scheduler) executePacketProcess(w *Worker, pkt gopacket.Packet) {
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
	if a := pkt.Layer(layers.LayerTypeARP); a != nil {
		arp := a.(*layers.ARP)
		srcIP = net.IP(arp.SourceProtAddress).String()
		srcMAC = strings.ToLower(net.HardwareAddr(arp.SourceHwAddress).String())
		if arp.Operation == layers.ARPReply {
			// ARP reply — definitive MAC→IP mapping
			dstIP = net.IP(arp.DstProtAddress).String()
		}
	}

	localSubnet := subnetBase(s.GeoFence.LocalIP)

	for _, mac := range []string{srcMAC, dstMAC} {
		if mac == "ff:ff:ff:ff:ff:ff" || isMulticastMAC(mac) {
			continue
		}
		d, ok := w.Devices[mac]
		if !ok {
			d = &Device{MAC: mac, Vendor: LookupOUI(mac), FirstSeen: now, LastSeen: now}
			w.Devices[mac] = d
		}
		d.PacketCount++
		d.ByteCount += pktLen
		d.LastSeen = now
		addProto(d, proto)
	}

	if d, ok := w.Devices[srcMAC]; ok {
		assignLocalIP(d, srcIP, localSubnet)
	}
	if d, ok := w.Devices[dstMAC]; ok {
		assignLocalIP(d, dstIP, localSubnet)
	}
}

// PrintStats logs per-worker statistics to stderr.
func (s *Scheduler) PrintStats() {
	for _, w := range s.Workers {
		fmt.Fprintf(os.Stderr,
			"  W%d: exec=%d stolen=%d arps=%d pkts=%d steal_ok=%d/%d\n",
			w.ID,
			atomic.LoadInt64(&w.Stats.TasksExecuted),
			atomic.LoadInt64(&w.Stats.TasksStolen),
			atomic.LoadInt64(&w.Stats.ARPsSent),
			atomic.LoadInt64(&w.Stats.PacketsProc),
			atomic.LoadInt64(&w.Stats.StealSuccess),
			atomic.LoadInt64(&w.Stats.StealAttempts),
		)
	}
}

// WorkerStatsSlice returns all worker stats for reporting.
func (s *Scheduler) WorkerStatsSlice() []WorkerStats {
	stats := make([]WorkerStats, len(s.Workers))
	for i, w := range s.Workers {
		stats[i] = WorkerStats{
			TasksExecuted: atomic.LoadInt64(&w.Stats.TasksExecuted),
			TasksStolen:   atomic.LoadInt64(&w.Stats.TasksStolen),
			ARPsSent:      atomic.LoadInt64(&w.Stats.ARPsSent),
			PacketsProc:   atomic.LoadInt64(&w.Stats.PacketsProc),
			StealAttempts: atomic.LoadInt64(&w.Stats.StealAttempts),
			StealSuccess:  atomic.LoadInt64(&w.Stats.StealSuccess),
		}
	}
	return stats
}

// RunWorkStealing is the top-level entry point for work-stealing scan.
func RunWorkStealing(ctx context.Context, opts ScanOpts) (*ScanResult, error) {
	if opts.Interface == "" {
		opts.Interface = "en0"
	}
	if opts.PacketCount == 0 {
		opts.PacketCount = 500
	}

	gf := BuildGeoFence(opts.Interface)

	sched, err := NewScheduler(opts.Interface, gf, runtime.NumCPU())
	if err != nil {
		return nil, err
	}
	defer sched.Close()

	// Distribute ARP sweep across workers
	sched.DistributeARPSweep()

	// Add mDNS + SSDP as global tasks
	sched.AddGlobalTask(Task{Kind: TaskSSDP})
	for _, svc := range []string{"_googlecast._tcp", "_waymo._tcp", "_grpc._tcp"} {
		sched.AddGlobalTask(Task{Kind: TaskMDNS, IP: svc})
	}

	// Mark feeding=true before starting workers so they know packets are coming
	sched.feeding.Store(true)

	// Start packet capture feeding into workers (concurrent)
	go func() {
		sched.DistributePackets(ctx, opts.PacketCount)
	}()

	// Run work-stealing loop
	result := sched.Run(ctx)

	// Print worker stats
	fmt.Fprintf(os.Stderr, "Worker stats:\n")
	sched.PrintStats()
	fmt.Fprintf(os.Stderr, "ARP listener devices: %d\n", len(sched.results))

	return result, nil
}
