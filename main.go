package main

import (
	"fmt"
	"log"
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

const emitSilenceDuration = time.Duration(10 * time.Millisecond)
const switchActiveLevel = uint(0) // active low or active high

type Debouncer struct {
	lastEmitTime  time.Time
	lastEmitLevel uint
}

func (d Debouncer) Push(level uint) bool {
	now := time.Now()
	if now.After(d.lastEmitTime.Add(emitSilenceDuration)) {
		d.lastEmitTime = now
		d.lastEmitLevel = level
		return true
	}
	return false
}

type Curtains struct {
	motor      *Motor
	watcher    *gpio.Watcher
	debouncing map[uint]Debouncer
	switchChan chan uint
	position   float32
	command    chan float32
	respond    chan struct{}
}

func NewCurtains() *Curtains {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer rpio.Close()

	rpioSwitchLeft := rpio.Pin(SwitchLeft)
	rpioSwitchRight := rpio.Pin(SwitchRight)
	rpioSwitchLeft.PullUp()
	rpioSwitchRight.PullUp()

	watcher := gpio.NewWatcher()
	watcher.AddPin(SwitchLeft)
	watcher.AddPin(SwitchRight)
	defer watcher.Close()

	motor := NewMotor(MotorLeft, MotorRight)

	c := &Curtains{
		motor:      motor,
		watcher:    watcher,
		debouncing: make(map[uint]Debouncer),
		switchChan: make(chan uint),
		position:   0,
		command:    make(chan float32),
		respond:    make(chan struct{}),
	}

	c.debouncing[SwitchLeft] = Debouncer{}
	c.debouncing[SwitchRight] = Debouncer{}

	go c.switchNotifier()
	go c.listen()

	return c
}

func (c *Curtains) switchNotifier() {
	for {
		pin, value := c.watcher.Watch()
		debouncer := c.debouncing[pin]
		if debouncer.Push(value) && value == switchActiveLevel {
			c.switchChan <- pin
		}
	}
}

func (c *Curtains) moveDuration(pos float32) time.Duration {
	// TODO actual time
	return time.Duration(30 * time.Second)
}

func (c *Curtains) moveDirection(pos float32) MotorDirection {
	if pos > c.position {
		return ClockwiseDirection
	}
	return CounterclockwiseDirection
}

func (c *Curtains) reckon(dur time.Duration, dir MotorDirection, reachedStop *uint) {
	if reachedStop != nil {
		switch *reachedStop {
		case SwitchLeft:
			c.position = 0
		case SwitchRight:
			c.position = 1
		default:
			panic("unrecognized hardstop reached")
		}
		log.Printf("position updated from hardstop, new position = %f", c.position)
		return
	}
	// TODO actual calculations
}

func (c *Curtains) move(newPosition float32) {
	d := c.moveDuration(newPosition)
	dir := c.moveDirection(newPosition)
	timer := time.NewTimer(d)
	defer timer.Stop()

	start := time.Now()
	c.motor.Move(dir)

	var reachedStop *uint

	select {
	case <-timer.C:
		// time ran out
	case *reachedStop = <-c.switchChan:
		// reached the hard stop
	}

	c.motor.Stop()
	dur := time.Now().Sub(start)
	c.reckon(dur, dir, reachedStop)

	time.Sleep(500 * time.Millisecond)
}

// we need listen so that we can catch switch changes that happen when the motor is off
func (c *Curtains) listen() {
	for {
		select {
		case newPos := <-c.command:
			c.move(newPos)
			c.respond <- struct{}{}
		case reachedStop := <-c.switchChan:
			c.reckon(0, StopDirection, &reachedStop)
		}
	}
}

func (c *Curtains) Move(newPosition float32) {
	c.command <- newPosition
	<-c.respond
}

func (c *Curtains) Close() {
	c.watcher.Close()
	rpio.Close()
}

func main() {
	c := NewCurtains()
	defer c.Close()

	char := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(char); err != nil {
			os.Exit(1)
		}
		quit := false
		switch char[0] {
		case 'r':
			c.Move(1)
		case 'l':
			c.Move(0)
		case 'x':
			quit = true
		}
		if quit {
			break
		}
	}
}
