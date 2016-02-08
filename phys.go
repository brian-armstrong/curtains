package main

import (
	"time"

	"github.com/brian-armstrong/gpio"
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
	StopDirection MotorDirection = iota
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

func (m *Motor) Move(dir MotorDirection) {
	switch dir {
	case StopDirection:
		m.Stop()
	case ClockwiseDirection:
		m.Clockwise()
	case CounterclockwiseDirection:
		m.Counterclockwise()
	default:
		panic("unrecognized motor direction")
	}
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

const emitSilenceDuration = time.Duration(1000 * time.Millisecond)
const switchActiveLevel = uint(0) // active low or active high

type Debouncer struct {
	lastEmitTime  time.Time
	lastEmitLevel uint
}

func (d *Debouncer) update(newTime time.Time, newLevel uint) {
	d.lastEmitTime = newTime
	d.lastEmitLevel = newLevel
}

func (d *Debouncer) Push(level uint) bool {
	now := time.Now()
	if d.lastEmitTime.IsZero() {
		d.update(now, level)
		return true
	}

	if level == d.lastEmitLevel {
		return false
	}

	if now.After(d.lastEmitTime.Add(emitSilenceDuration)) {
		d.update(now, level)
		return true
	}

	return false
}
