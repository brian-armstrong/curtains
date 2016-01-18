package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"os"
	"time"
)

const (
	GPIO22 rpio.Pin = 22  // header pin 15
	GPIO23 = 23  // header pin 16
	GPIO24 = 24  // header pin 18
	GPIO27 = 27  // header pin 13
)

const (
	MotorLeftGround rpio.Pin = GPIO27
	MotorLeftVCC = GPIO22
	MotorRightGround = GPIO23
	MotorRightGround = GPIO24
)

func main() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer rpio.Close()

	pin := MotorLeftVCC

	pin.Output()

	for x := 0; x < 20; x++ {
		pin.Toggle()
		time.Sleep(time.Second)
	}
}
