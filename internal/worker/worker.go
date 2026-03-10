package worker

import (
	"context"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/awandataindonesia/cekping-agent/internal/config"
	"github.com/awandataindonesia/cekping-agent/internal/executor"
	"github.com/awandataindonesia/cekping-agent/pkg/protocol"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Worker struct {
	cfg          *config.Config
	activeTasks  map[string]context.CancelFunc
	targetToTask map[string]string
	tasksMu      sync.Mutex
}

func NewWorker(cfg *config.Config) *Worker {
	return &Worker{
		cfg:          cfg,
		activeTasks:  make(map[string]context.CancelFunc),
		targetToTask: make(map[string]string),
	}
}

func (w *Worker) Start() {
	hostname, _ := os.Hostname()
	log.Printf("Cekping Agent starting... (PID: %d, Host: %s)", os.Getpid(), hostname)
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
	var opts []grpc.DialOption
	if w.cfg.Secure {
		creds := credentials.NewClientTLSFromCert(nil, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(w.cfg.ServerAddr, opts...)
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

	// Create thread-safe stream wrapper
	safeStream := &ThreadSafeStream{
		stream: stream,
	}

	// 1. Auth
	if err := safeStream.Send(&protocol.AgentMsg{
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
			if err := safeStream.Send(&protocol.AgentMsg{
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

		go w.handleMessage(safeStream, msg)
	}
}

// ThreadSafeStream wraps a grpc client stream with a mutex to prevent data races during concurrent Sends.
type ThreadSafeStream struct {
	stream protocol.PingveService_ConnectClient
	mu     sync.Mutex
}

func (s *ThreadSafeStream) Send(msg *protocol.AgentMsg) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stream.Send(msg)
}

func (w *Worker) handleMessage(stream *ThreadSafeStream, msg *protocol.ServerMsg) {
	switch payload := msg.Payload.(type) {
	case *protocol.ServerMsg_PingTask:
		task := payload.PingTask
		log.Printf("Received Ping Task: %s (ID: %s)", task.Target, task.Id)

		ctx, cancel := context.WithCancel(context.Background())
		w.tasksMu.Lock()
		w.activeTasks[task.Id] = cancel
		w.tasksMu.Unlock()

		defer func() {
			w.tasksMu.Lock()
			delete(w.activeTasks, task.Id)
			w.tasksMu.Unlock()
		}()

		finalStats, err := executor.DoPing(ctx, task.Target, int(task.Count), func(seq, ttl int, rtt float64) {
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
		log.Printf("Received MTR Task: %s (ID: %s)", task.Target, task.Id)

		ctx, cancel := context.WithCancel(context.Background())
		w.tasksMu.Lock()
		w.activeTasks[task.Id] = cancel
		w.tasksMu.Unlock()

		defer func() {
			w.tasksMu.Lock()
			delete(w.activeTasks, task.Id)
			w.tasksMu.Unlock()
		}()

		err := executor.DoMTR(ctx, task.Target, int(task.Count), func(stats executor.MTRHopStats) {
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
			_ = stream.Send(&protocol.AgentMsg{
				Payload: &protocol.AgentMsg_MtrResult{
					MtrResult: &protocol.MTRResult{
						TaskId:    task.Id,
						Target:    task.Target,
						RawOutput: err.Error(),
						IsFinal:   true,
					},
				},
			})
			return
		}

		_ = stream.Send(&protocol.AgentMsg{
			Payload: &protocol.AgentMsg_MtrResult{
				MtrResult: &protocol.MTRResult{
					TaskId:  task.Id,
					Target:  task.Target,
					IsFinal: true,
				},
			},
		})

	case *protocol.ServerMsg_CancelTask:
		task := payload.CancelTask
		log.Printf("Received Cancel Request for Task ID: %s", task.Id)
		w.tasksMu.Lock()
		if cancel, ok := w.activeTasks[task.Id]; ok {
			log.Printf("Executing cancellation for Task ID: %s", task.Id)
			cancel()
			delete(w.activeTasks, task.Id)
		}
		w.tasksMu.Unlock()
	}
}
