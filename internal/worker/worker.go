package worker

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/awandata/pingve-agent/internal/config"
	"github.com/awandata/pingve-agent/internal/executor"
	"github.com/awandata/pingve-agent/pkg/protocol"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Worker struct {
	cfg *config.Config
}

func NewWorker(cfg *config.Config) *Worker {
	return &Worker{cfg: cfg}
}

func (w *Worker) Start() {
	backoff := 1 * time.Second
	for {
		log.Printf("Connecting to server at %s...", w.cfg.ServerAddr)
		err := w.connectAndLoop()
		log.Printf("Disconnected: %v. Retrying in %v...", err, backoff)

		time.Sleep(backoff)
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (w *Worker) connectAndLoop() error {
	conn, err := grpc.Dial(w.cfg.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := protocol.NewPingveServiceClient(conn)
	stream, err := client.Connect(context.Background())
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown-agent"
	}

	// 1. Auth
	if err := stream.Send(&protocol.AgentMsg{
		Payload: &protocol.AgentMsg_Auth{
			Auth: &protocol.AuthRequest{
				Token:    w.cfg.Token,
				Hostname: hostname,
				Version:  "1.0.1", // Increment for tracking
			},
		},
	}); err != nil {
		return err
	}

	// 2. Wait for Auth Response
	ack, err := stream.Recv()
	if err != nil {
		return err
	}
	authResp := ack.GetAuth()
	if authResp == nil || !authResp.Success {
		return log.Output(1, "Authentication failed: "+authResp.GetErrorMessage())
	}
	log.Println("Authenticated successfully")

	// 3. Heartbeat Loop
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := stream.Send(&protocol.AgentMsg{
				Payload: &protocol.AgentMsg_Heartbeat{
					Heartbeat: &protocol.Heartbeat{
						Timestamp: time.Now().Unix(),
						Uptime:    0, // Get uptime
					},
				},
			}); err != nil {
				return
			}
		}
	}()

	// 4. Task Loop
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		go w.handleMessage(stream, msg)
	}
}

func (w *Worker) handleMessage(stream protocol.PingveService_ConnectClient, msg *protocol.ServerMsg) {
	switch payload := msg.Payload.(type) {
	case *protocol.ServerMsg_PingTask:
		task := payload.PingTask
		log.Printf("Received Ping Task: %s", task.Target)
		finalStats, err := executor.DoPing(task.Target, int(task.Count), func(seq, ttl int, rtt float64) {
			// Stream result per packet
			_ = stream.Send(&protocol.AgentMsg{
				Payload: &protocol.AgentMsg_PingResult{
					PingResult: &protocol.PingResult{
						TaskId:  task.Id,
						Target:  task.Target,
						Seq:     int32(seq),
						Ttl:     int32(ttl),
						Time:    rtt,
						IsFinal: false,
					},
				},
			})
		})

		if err != nil {
			log.Printf("Ping error: %v", err)
		}

		if finalStats != nil {
			// Send Final Summary
			_ = stream.Send(&protocol.AgentMsg{
				Payload: &protocol.AgentMsg_PingResult{
					PingResult: &protocol.PingResult{
						TaskId:  task.Id,
						Target:  task.Target,
						Sent:    int32(len(finalStats.Rtts)),
						MinRtt:  finalStats.Min,
						MaxRtt:  finalStats.Max,
						AvgRtt:  finalStats.Avg,
						StdDev:  finalStats.StdDev,
						Rtts:    finalStats.Rtts,
						IsFinal: true,
					},
				},
			})
		}

	case *protocol.ServerMsg_MtrTask:
		task := payload.MtrTask
		log.Printf("Received MTR Task: %s", task.Target)

		err := executor.DoMTR(task.Target, func(stats executor.MTRHopStats) {
			_ = stream.Send(&protocol.AgentMsg{
				Payload: &protocol.AgentMsg_MtrResult{
					MtrResult: &protocol.MTRResult{
						TaskId: task.Id,
						Target: task.Target,
						Hop: &protocol.MTRHop{
							Hop:   int32(stats.Hop),
							Ip:    stats.IP,
							Loss:  stats.Loss,
							Sent:  int32(stats.Sent),
							Last:  stats.Last,
							Avg:   stats.Avg,
							Best:  stats.Best,
							Worst: stats.Worst,
							Stdev: stats.StdDev,
						},
						IsFinal: false,
					},
				},
			})
		})

		if err != nil {
			log.Printf("MTR error: %v", err)
			return
		}

		// Send Final Confirmation
		_ = stream.Send(&protocol.AgentMsg{
			Payload: &protocol.AgentMsg_MtrResult{
				MtrResult: &protocol.MTRResult{
					TaskId:  task.Id,
					Target:  task.Target,
					IsFinal: true,
				},
			},
		})

	}
}
