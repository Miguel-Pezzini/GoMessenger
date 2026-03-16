package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

type Service struct {
	Name string
	Path string
}

func main() {
	services := []Service{
		{"auth_service", "./services/auth_service/cmd"},
		{"chat_service", "./services/chat_service/cmd"},
		{"friends_service", "./services/friends_service/cmd"},
		{"gateway", "./services/gateway/cmd"},
		{"websocket_service", "./services/websocket_service/cmd"},
	}

	var processes []*exec.Cmd

	fmt.Println("Starting the servicces...\n")

	for _, s := range services {
		fmt.Printf("-> %s Start\n", s.Name)

		cmd := exec.Command("go", "run", ".")
		cmd.Dir = s.Path

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		if err != nil {
			fmt.Printf("Error to start %s: %v\n", s.Name, err)
			continue
		}

		processes = append(processes, cmd)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	fmt.Println("\nEncerrando serviços...")

	for _, p := range processes {
		if p.Process == nil {
			continue
		}

		if runtime.GOOS == "windows" {
			_ = p.Process.Kill()
		} else {
			_ = p.Process.Signal(syscall.SIGTERM)
		}
	}

	fmt.Println("Todos os serviços foram encerrados.")
}
