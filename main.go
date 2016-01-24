package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/stianeikeland/go-rpio"
)

const (
	GPIO17 rpio.Pin = 17 // header pin 11
	GPIO22          = 22 // header pin 15
	GPIO23          = 23 // header pin 16
	GPIO27          = 27 // header pin 13
)

const (
	MotorLeft   rpio.Pin = GPIO22
	MotorRight           = GPIO23
	SwitchLeft  rpio.Pin = GPIO17
	SwitchRight rpio.Pin = GPIO27
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
		fmt.Printf("failed to open gpio export file for writing\n")
		os.Exit(1)
	}
	defer export.Close()

	export.Write([]byte(strconv.Itoa(int(p))))

	dir, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", p), os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("failed to open gpio %d direction file for writing\n", p)
		os.Exit(1)
	}
	defer dir.Close()

	dir.Write([]byte("in"))

	edge, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", p), os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("failed to open gpio %d edge file for writing\n", p)
		os.Exit(1)
	}
	defer edge.Close()

	edge.Write([]byte("both"))
}

func watchGPIO(p rpio.Pin) chan []byte {
	exportGPIO(p)
	value, err := os.Open(fmt.Sprintf("/sys/class/gpio/gpio%d/value", p))
	if err != nil {
		fmt.Printf("failed to open gpio %d value file for writing\n", p)
		os.Exit(1)
	}
	c := make(chan []byte)
	go func() {
		defer value.Close()
		for {
			b, err := ioutil.ReadAll(value)
			if err != nil {
				os.Exit(1)
			}
			c <- b
		}
	}()
	return c
}

func main() {
	leftChan := watchGPIO(SwitchLeft)
	rightChan := watchGPIO(SwitchRight)

	go func() {
		for {
			select {
			case b := <-leftChan:
				fmt.Printf("read %s from left switch\n", string(b))
			case b := <-rightChan:
				fmt.Printf("read %s from right switch\n", string(b))
			}
		}
	}()

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
