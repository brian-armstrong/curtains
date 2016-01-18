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
	MotorRightVCC = GPIO24
)

type TerminalActivation int

const (
	NeitherActivated TerminalActivation = 0
	LeftActivated
	RightActivated
)

type MotorTerminal struct {
	ground rpio.Pin
	vcc rpio.Pin
	activation TerminalActivation
}

func NewMotorTerminal(ground rpio.Pin, vcc rpio.Pin) *MotorTerminal {
	mt := MotorTerminal {
		ground: ground,
		vcc: vcc,
		activation: NeitherActivated,
	}

	mt.ground.Low()
	mt.ground.Output()

	mt.vcc.Low()
	mt.vcc.Output()

	return &mt
}

func (mt *MotorTerminal) Ground() {
	mt.vcc.Low()
	mt.ground.High()
}

func (mt *MotorTerminal) VCC() {
	mt.ground.Low()
	mt.vcc.High()
}

func (mt *MotorTerminal) Disconnect() {
	mt.ground.Low()
	mt.vcc.Low()
}

func main() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer rpio.Close()

	leftTerm := NewMotorTerminal(MotorLeftGround, MotorLeftVCC)
	rightTerm := NewMotorTerminal(MotorRightGround, MotorRightVCC)

	for x := 0; x < 20; x++ {
		leftTerm.Ground()
		rightTerm.VCC()
		time.Sleep(time.Second)
		rightTerm.Ground()
		leftTerm.VCC()
		time.Sleep(time.Second)
	}
}
