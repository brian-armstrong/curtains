package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brian-armstrong/gpio"

	"github.com/stianeikeland/go-rpio"
)

const (
	GPIO17 uint = 17 // header pin 11
	GPIO22 uint = 22 // header pin 15
	GPIO23 uint = 23 // header pin 16
	GPIO27 uint = 27 // header pin 13
)

const (
	MotorLeft   uint = GPIO22
	MotorRight  uint = GPIO23
	SwitchLeft  uint = GPIO17
	SwitchRight uint = GPIO27
)

type MotorDirection int

const (
	StopDirection MotorDirection = 0
	ClockwiseDirection
	CounterclockwiseDirection
)

type Motor struct {
	left      gpio.Pin
	right     gpio.Pin
	direction MotorDirection
}

func NewMotor(left uint, right uint) *Motor {
	m := Motor{
		left:      gpio.NewOutput(left, true),
		right:     gpio.NewOutput(right, true),
		direction: StopDirection,
	}

	return &m
}

func (m *Motor) Clockwise() {
	m.left.Low()
	m.right.High()
	m.direction = ClockwiseDirection
}

func (m *Motor) Counterclockwise() {
	m.left.High()
	m.right.Low()
	m.direction = CounterclockwiseDirection
}

func (m *Motor) Stop() {
	m.left.High()
	m.right.High()
	m.direction = StopDirection
}
func main() {
	watcher := gpio.NewWatcher()
	watcher.AddPin(SwitchLeft)
	watcher.AddPin(SwitchRight)
	defer watcher.Close()

	go func() {
		for {
			p, v := watcher.Watch()
			fmt.Printf("read %d from gpio %d\n", v, p)
		}
	}()

	motor := NewMotor(MotorLeft, MotorRight)

	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer rpio.Close()

	rpioSwitchLeft := rpio.Pin(SwitchLeft)
	rpioSwitchRight := rpio.Pin(SwitchRight)
	rpioSwitchLeft.PullUp()
	rpioSwitchRight.PullUp()

	for x := 0; x < 5; x++ {
		motor.Clockwise()
		time.Sleep(time.Second)
		motor.Stop()
		time.Sleep(500 * time.Millisecond)
		motor.Counterclockwise()
		time.Sleep(time.Second)
		motor.Stop()
		time.Sleep(500 * time.Millisecond)
	}
}
