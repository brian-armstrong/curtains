package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"os"
	"time"
)

const (
	rpio.Pin GPIO22 = 22  // header pin 15
	rpio.Pin GPIO23 = 23  // header pin 16
	rpio.Pin GPIO24 = 24  // header pin 18
	rpio.Pin GPIO27 = 27  // header pin 13
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
