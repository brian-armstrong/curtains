package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brian-armstrong/gpio"

	"github.com/stianeikeland/go-rpio"
)

const (
	GPIO17 rpio.Pin = 17 // header pin 11
	GPIO22 rpio.Pin = 22 // header pin 15
	GPIO23 rpio.Pin = 23 // header pin 16
	GPIO27 rpio.Pin = 27 // header pin 13
)

const (
	MotorLeft   rpio.Pin = GPIO22
	MotorRight           = GPIO23
	SwitchLeft           = GPIO17
	SwitchRight          = GPIO27
)

type MotorDirection int

const (
	StopDirection MotorDirection = 0
	ClockwiseDirection
	CounterclockwiseDirection
)

type Motor struct {
	left      rpio.Pin
	right     rpio.Pin
	direction MotorDirection
}

func NewMotor(left rpio.Pin, right rpio.Pin) *Motor {
	m := Motor{
		left:      left,
		right:     right,
		direction: StopDirection,
	}

	m.left.High()
	m.right.High()

	m.left.Output()
	m.right.Output()

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
	watcher.AddPin(uint(SwitchLeft))
	watcher.AddPin(uint(SwitchRight))
	defer watcher.Close()

	go func() {
		for {
			p, v := watcher.Watch()
			fmt.Printf("read %d from gpio %d\n", v, p)
		}
	}()

	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer rpio.Close()

	motor := NewMotor(MotorLeft, MotorRight)

	SwitchLeft.PullUp()
	SwitchRight.PullUp()

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
