package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"os"
	"time"
)

const (
	GPIO22 rpio.Pin = 22  // header pin 15
	GPIO23 rpio.Pin = 23  // header pin 16
	GPIO24 rpio.Pin = 24  // header pin 18
	GPIO27 rpio.Pin = 27  // header pin 13
)

func main() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer rpio.Close()

	pin := rpio.Pin(GPIO22)

	pin.Output()

	for x := 0; x < 20; x++ {
		pin.Toggle()
		time.Sleep(time.Second)
	}
}
