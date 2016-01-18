package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"os"
	"time"
)

func main() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer rpio.Close()

	pin := rpio.Pin(22)

	pin.Output()

	for x := 0; x < 20; x++ {
		pin.Toggle()
		time.Sleep(time.Second)
	}
}
