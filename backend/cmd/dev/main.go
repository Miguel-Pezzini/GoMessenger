package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

type service struct {
	name string
	path string
}

func main() {
	services := []service{
		{name: "auth", path: "./services/auth/cmd"},
		{name: "chat", path: "./services/chat/cmd"},
		{name: "friends", path: "./services/friends/cmd"},
		{name: "gateway", path: "./services/gateway/cmd"},
		{name: "logging", path: "./services/logging/cmd"},
		{name: "notification", path: "./services/notification/cmd"},
		{name: "presence", path: "./services/presence_service/cmd"},
		{name: "websocket", path: "./services/websocket/cmd"},
	}

	processes := make([]*exec.Cmd, 0, len(services))
	fmt.Println("starting services")

	for _, svc := range services {
		fmt.Printf("-> %s\n", svc.name)

		cmd := exec.Command("go", "run", ".")
		cmd.Dir = svc.path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Printf("error starting %s: %v\n", svc.name, err)
			continue
		}
		processes = append(processes, cmd)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	for _, proc := range processes {
		if proc.Process == nil {
			continue
		}
		if runtime.GOOS == "windows" {
			_ = proc.Process.Kill()
			continue
		}
		_ = proc.Process.Signal(syscall.SIGTERM)
	}
}
