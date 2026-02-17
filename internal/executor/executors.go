package executor

import (
	"math"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	probing "github.com/prometheus-community/pro-bing"
)

type PingStats struct {
	Min, Max, Avg, StdDev float64
	PacketLoss            float64
	Rtts                  []float64
}

func DoPing(target string, count int, onPacket func(seq, ttl int, rtt float64)) (*PingStats, error) {
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return nil, err
	}
	pinger.Count = count
	pinger.Interval = 1 * time.Second // 1 second as per plan
	pinger.Timeout = 15 * time.Second

	pinger.SetPrivileged(true)

	pinger.OnRecv = func(pkt *probing.Packet) {
		rttVal := float64(pkt.Rtt.Seconds() * 1000)
		onPacket(pkt.Seq, pkt.TTL, rttVal)
	}

	err = pinger.Run()
	if err != nil {
		return nil, err
	}

	stats := pinger.Statistics()
	rtts := make([]float64, len(stats.Rtts))
	for i, d := range stats.Rtts {
		rtts[i] = float64(d.Seconds() * 1000)
	}

	return &PingStats{
		Min:        float64(stats.MinRtt.Seconds() * 1000),
		Max:        float64(stats.MaxRtt.Seconds() * 1000),
		Avg:        float64(stats.AvgRtt.Seconds() * 1000),
		StdDev:     float64(stats.StdDevRtt.Seconds() * 1000),
		PacketLoss: stats.PacketLoss,
		Rtts:       rtts,
	}, nil
}

type MTRHopStats struct {
	Hop     int
	IP      string
	Sent    int
	Loss    float64 // Percentage
	Last    float64 // ms
	Avg     float64 // ms
	Best    float64 // ms
	Worst   float64 // ms
	StdDev  float64 // ms
	Dropped int
	Rtts    []float64
}

func DoMTR(target string, onHop func(MTRHopStats)) error {
	// 1. Resolve Target
	dst, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return err
	}

	// 2. Open PacketConn (ICMP)
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return err
	}
	defer c.Close()

	// 3. MTR Configuration
	const maxHops = 30
	const cycles = 20 // Default count
	timeout := 1 * time.Second

	// State
	hops := make(map[int]*MTRHopStats) // hopIdx -> Stats
	// Initialize hops
	for i := 1; i <= maxHops; i++ {
		hops[i] = &MTRHopStats{Hop: i, Best: 999999, Worst: 0}
	}

	// 4. Run Cycles
	id := os.Getpid() & 0xffff

	for seq := 1; seq <= cycles; seq++ {
		// Stop if we reached target in previous cycle?
		// MTR continues probing all hops even if target found at hop X.
		// But in discovery, we don't know X.
		// We'll just run 1..30. If we hit target at 5, hops 6..30 will timeout (or we stop sending if we know target is at 5).
		// Let's track `maxDiscoveredHop`.

		targetReachedAt := 0

		for ttl := 1; ttl <= maxHops; ttl++ {
			if targetReachedAt > 0 && ttl > targetReachedAt {
				break
			}

			// Construct Message
			wm := icmp.Message{
				Type: ipv4.ICMPTypeEcho, Code: 0,
				Body: &icmp.Echo{
					ID: id, Seq: (seq << 8) | ttl, // Encode cycle & ttl in seq? Or just use global seq
					Data: []byte("PingVe-MTR"),
				},
			}
			wb, err := wm.Marshal(nil)
			if err != nil {
				continue
			}

			// Set TTL
			pConn := c.IPv4PacketConn()
			if pConn != nil {
				pConn.SetTTL(ttl)
			}

			// Send
			start := time.Now()
			if _, err := c.WriteTo(wb, dst); err != nil {
				continue
			}

			// Update Sent count
			hops[ttl].Sent++

			// Read Reply (with timeout)
			c.SetReadDeadline(time.Now().Add(timeout))
			rb := make([]byte, 1500)
			n, peer, err := c.ReadFrom(rb)

			rtt := time.Since(start).Seconds() * 1000 // ms

			if err != nil {
				// Recalculate Loss
				hops[ttl].Loss = float64(hops[ttl].Sent-len(hops[ttl].Rtts)) / float64(hops[ttl].Sent) * 100
				onHop(*hops[ttl])
				continue
			}

			// Parse Reply
			rm, err := icmp.ParseMessage(1, rb[:n])
			if err != nil {
				continue
			}

			// Check Type
			isReply := false

			switch rm.Type {
			case ipv4.ICMPTypeTimeExceeded:
				// Valid Hop response
			case ipv4.ICMPTypeEchoReply:
				isReply = true
			default:
				// Ignore others
				continue
			}

			// Update Hop Stats
			h := hops[ttl]
			h.IP = peer.String() // The responder
			h.Rtts = append(h.Rtts, rtt)
			h.Last = rtt

			// Min/Max/Avg/StdDev
			if rtt < h.Best {
				h.Best = rtt
			}
			if rtt > h.Worst {
				h.Worst = rtt
			}

			sum := 0.0
			for _, v := range h.Rtts {
				sum += v
			}
			h.Avg = sum / float64(len(h.Rtts))

			if len(h.Rtts) > 1 {
				variance := 0.0
				for _, v := range h.Rtts {
					variance += math.Pow(v-h.Avg, 2)
				}
				h.StdDev = math.Sqrt(variance / float64(len(h.Rtts)-1)) // -1 sample
			}

			// Recalculate Loss
			h.Loss = float64(h.Sent-len(h.Rtts)) / float64(h.Sent) * 100

			onHop(*h)

			if isReply {
				targetReachedAt = ttl
			}
		}

		// Wait a bit between cycles?
		time.Sleep(500 * time.Millisecond) // 0.5s interval
	}

	return nil
}
