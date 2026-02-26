package executor

import (
	"math"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type PingStats struct {
	Min, Max, Avg, StdDev float64
	PacketLoss            float64
	Rtts                  []float64
}

func DoPing(target string, count int, onPacket func(seq, ttl int, rtt float64)) (*PingStats, error) {
	// 1. Resolve Target
	dst, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return nil, err
	}

	// 2. Open PacketConn (ICMP)
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	defer c.Close()

	// 3. Stats Tracking
	stats := &PingStats{
		Min: 999999,
		Max: 0,
	}
	sent := 0
	received := 0

	// Unique ID masked to 16 bits (0-65535) to match ICMP spec
	id := ((os.Getpid() & 0x7fff) + int(time.Now().UnixNano()&0x7fff)) & 0xffff
	timeout := 1 * time.Second

	for seq := 1; seq <= count; seq++ {
		// Construct Message
		wm := icmp.Message{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmp.Echo{
				ID: id, Seq: seq,
				Data: []byte("PingVe-Ping"),
			},
		}
		wb, err := wm.Marshal(nil)
		if err != nil {
			time.Sleep(timeout) // simulate delay on error
			continue
		}

		// Send
		start := time.Now()
		sent++
		if _, err := c.WriteTo(wb, dst); err != nil {
			onPacket(seq, 0, 0)
			time.Sleep(timeout)
			continue
		}

		// Read Reply (with timeout)
		var rtt float64
		deadline := time.Now().Add(timeout)
		for {
			c.SetReadDeadline(deadline)
			rb := make([]byte, 1500)
			n, _, err := c.ReadFrom(rb)

			if err != nil {
				// Timeout (RTO) or socket error
				onPacket(seq, 0, 0)
				goto WaitInterval
			}

			// Parse Reply
			rm, err := icmp.ParseMessage(1, rb[:n])
			if err != nil {
				continue // Ignore parse failures, wait for next packet
			}

			if rm.Type == ipv4.ICMPTypeEchoReply {
				if echo, ok := rm.Body.(*icmp.Echo); ok {
					if echo.ID == id && echo.Seq == seq {
						// Success! We found our specific packet
						rtt = time.Since(start).Seconds() * 1000 // ms
						received++
						stats.Rtts = append(stats.Rtts, rtt)
						if rtt < stats.Min {
							stats.Min = rtt
						}
						if rtt > stats.Max {
							stats.Max = rtt
						}
						// Fire callback
						onPacket(seq, 0, rtt)
						break
					}
				}
			}
		}

	WaitInterval:
		// Wait remainder of the 1-second interval
		elapsed := time.Since(start)
		if elapsed < time.Second {
			time.Sleep(time.Second - elapsed)
		}
	}

	// Finalize Stats
	if received > 0 {
		sum := 0.0
		for _, v := range stats.Rtts {
			sum += v
		}
		stats.Avg = sum / float64(received)

		if received > 1 {
			variance := 0.0
			for _, v := range stats.Rtts {
				variance += math.Pow(v-stats.Avg, 2)
			}
			stats.StdDev = math.Sqrt(variance / float64(received-1))
		}
	} else {
		stats.Min = 0
	}

	stats.PacketLoss = float64(sent-received) / float64(sent) * 100

	return stats, nil
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
	// Unique ID masked to 16 bits
	id := ((os.Getpid() & 0x7fff) + int(time.Now().UnixNano()&0x7fff)) & 0xffff
	maxDiscoveredHop := maxHops

	for seq := 1; seq <= cycles; seq++ {
		for ttl := 1; ttl <= maxHops; ttl++ {
			if ttl > maxDiscoveredHop {
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
			var rtt float64
			deadline := time.Now().Add(timeout)
			for {
				c.SetReadDeadline(deadline)
				rb := make([]byte, 1500)
				n, peer, err := c.ReadFrom(rb)

				if err != nil {
					// Timeout / Error: Packet lost
					goto UpdateStats
				}

				// Parse Reply
				rm, err := icmp.ParseMessage(1, rb[:n])
				if err != nil {
					continue // Corrupt packet: wait for another
				}

				// Check Type
				isReply := false
				switch rm.Type {
				case ipv4.ICMPTypeTimeExceeded:
					// Strict filter: Check if the encapsulated original packet matches our MTR packet
					if timeExceeded, ok := rm.Body.(*icmp.TimeExceeded); ok {
						if len(timeExceeded.Data) >= 28 { // IPv4 header (20) + ICMP header (8)
							// Extract inner ICMP Identifier (bytes 24-25 in total data)
							innerID := int(timeExceeded.Data[24])<<8 | int(timeExceeded.Data[25])
							if innerID != id {
								continue // Not our packet
							}
						}
					}
					// Valid Hop response
				case ipv4.ICMPTypeEchoReply:
					if echo, ok := rm.Body.(*icmp.Echo); ok {
						if echo.ID != id || echo.Seq != ((seq<<8)|ttl) {
							continue // Not our specific echo reply
						}
					}
					isReply = true
				default:
					// Ignore others
					continue
				}

				rtt = time.Since(start).Seconds() * 1000 // ms
				h := hops[ttl]

				// Success: Update Hop Stats
				if h.IP == "" {
					h.IP = peer.String() // The responder
				}
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

				if isReply {
					if ttl < maxDiscoveredHop {
						maxDiscoveredHop = ttl
					}
				}
				break // Successfully parsed a valid packet for this TTL
			}

		UpdateStats:
			h := hops[ttl]
			// Recalculate Loss based on Sent vs Received
			h.Loss = float64(h.Sent-len(h.Rtts)) / float64(h.Sent) * 100

			// Always trigger onHop so UI gets the timeout/RTO update
			onHop(*h)
		}

		// Wait a bit between cycles?
		time.Sleep(500 * time.Millisecond) // 0.5s interval
	}

	return nil
}
