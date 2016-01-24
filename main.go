package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"os"
	"strconv"
	"time"
)

const (
	GPIO22 rpio.Pin = 22 // header pin 15
	GPIO23          = 23 // header pin 16
)

const (
	MotorLeft  rpio.Pin = GPIO22
	MotorRight          = GPIO23
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

func exportGPIO(p rpio.Pin) {
	export, err := os.OpenFile("/sys/class/gpio/export", os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(1)
	}
	defer export.Close()

	export.Write([]byte(strconv.Itoa(int(p))))

	dir, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", p), os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(1)
	}
	defer dir.Close()

	dir.Write([]byte("in"))

	edge, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", p), os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(1)
	}
	defer edge.Close()

	edge.Write([]byte("both"))
}

func main() {
	exportGPIO(GPIO22)

	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer rpio.Close()

	motor := NewMotor(MotorLeft, MotorRight)

	for x := 0; x < 20; x++ {
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
