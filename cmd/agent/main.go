package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/awandataindonesia/cekping-agent/internal/config"
	"github.com/awandataindonesia/cekping-agent/internal/worker"
)

const serviceTemplate = `[Unit]
Description=PingVe Agent
After=network.target

[Service]
ExecStart=/usr/local/bin/pingve-agent
Restart=always
User=root
Environment=AGENT_TOKEN={{TOKEN}}
Environment=SERVER_ADDR={{SERVER}}
Environment=AGENT_HOSTNAME={{HOSTNAME}}

[Install]
WantedBy=multi-user.target
`

func main() {
	// Flags
	install := flag.Bool("install", false, "Install the agent as a systemd service")
	token := flag.String("token", "", "Agent Token (required for install)")
	server := flag.String("server", "localhost:50051", "Server Address (for install)")
	hostName := flag.String("host", "", "Hostname (optional for install)")
	flag.Parse()

	if *install {
		runInstall(*token, *server, *hostName)
		return
	}

	log.Println("Starting PingVe Agent...")
	cfg := config.LoadConfig()

	w := worker.NewWorker(cfg)
	w.Start()
}

func runInstall(token, server, hostArg string) {
	if token == "" {
		log.Fatal("Error: -token is required for installation")
	}

	// Check Root
	if os.Geteuid() != 0 {
		log.Fatal("Error: Installation requires root privileges (sudo)")
	}

	binPath := "/usr/local/bin/pingve-agent"
	servicePath := "/etc/systemd/system/pingve-agent.service"

	// 1. Copy Binary
	log.Printf("Installing binary to %s...", binPath)
	selfPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to locate self: %v", err)
	}

	// Stop service first if running
	_ = exec.Command("systemctl", "stop", "pingve-agent").Run()

	input, err := os.ReadFile(selfPath)
	if err != nil {
		log.Fatalf("Failed to read self: %v", err)
	}
	if err := os.WriteFile(binPath, input, 0755); err != nil {
		log.Fatalf("Failed to copy binary: %v", err)
	}

	// 2. Create Service File
	log.Println("Creating systemd service...")
	finalHost := hostArg
	if finalHost == "" {
		finalHost, _ = os.Hostname()
	}

	content := strings.ReplaceAll(serviceTemplate, "{{TOKEN}}", token)
	content = strings.ReplaceAll(content, "{{SERVER}}", server)
	content = strings.ReplaceAll(content, "{{HOSTNAME}}", finalHost)

	if err := os.WriteFile(servicePath, []byte(content), 0644); err != nil {
		log.Fatalf("Failed to create service file: %v", err)
	}

	// 3. Enable & Start
	log.Println("Enabling and starting service...")
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		log.Fatalf("Daemon reload failed: %v", err)
	}
	if err := exec.Command("systemctl", "enable", "--now", "pingve-agent").Run(); err != nil {
		log.Fatalf("Failed to enable service: %v", err)
	}

	log.Println("Installation Successful! Service 'pingve-agent' is running.")
}
