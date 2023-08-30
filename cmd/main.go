package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alperenunal/mesaj/mqtt"
)

var quitChan = make(chan os.Signal, 1)

func main() {
	signal.Notify(quitChan, syscall.SIGTERM)

	err := mqtt.Tcp(mqtt.DefaultAcl())
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Running")
	<-quitChan
}
