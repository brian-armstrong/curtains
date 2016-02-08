package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/brian-armstrong/gpio"
	"github.com/stianeikeland/go-rpio"
)

type Controller struct {
	motor      *Motor
	watcher    *gpio.Watcher
	debouncing map[uint]*Debouncer
	switchChan chan uint
	position   float64
	command    chan float64
	respond    chan struct{}
}

func NewController() *Controller {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rpioSwitchLeft := rpio.Pin(SwitchLeft)
	rpioSwitchRight := rpio.Pin(SwitchRight)
	rpioSwitchLeft.PullUp()
	rpioSwitchRight.PullUp()

	watcher := gpio.NewWatcher()
	watcher.AddPin(SwitchLeft)
	watcher.AddPin(SwitchRight)

	motor := NewMotor(MotorLeft, MotorRight)

	c := &Controller{
		motor:      motor,
		watcher:    watcher,
		debouncing: make(map[uint]*Debouncer),
		switchChan: make(chan uint),
		position:   0,
		command:    make(chan float64),
		respond:    make(chan struct{}),
	}

	c.debouncing[SwitchLeft] = &Debouncer{}
	c.debouncing[SwitchRight] = &Debouncer{}

	go c.switchNotifier()
	go c.listen()

	return c
}

func (c *Controller) switchNotifier() {
	for {
		pin, value := c.watcher.Watch()
		debouncer := c.debouncing[pin]
		if debouncer.Push(value) && value == switchActiveLevel {
			c.switchChan <- pin
		}
	}
}

func (c *Controller) moveDuration(newPos float64) time.Duration {
	// TODO actual time
	return time.Duration(math.Abs(newPos-c.position)*30) * time.Second
}

func (c *Controller) moveDirection(newPos float64) MotorDirection {
	if c.position == 1 {
		return ClockwiseDirection
	}

	if c.position == 0 {
		return CounterclockwiseDirection
	}

	if newPos > c.position {
		return ClockwiseDirection
	}

	return CounterclockwiseDirection
}

func (c *Controller) reckon(dur time.Duration, dir MotorDirection, reachedStop *uint) {
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

func (c *Controller) move(newPosition float64) {
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
	case stop := <-c.switchChan:
		// reached the hard stop
		reachedStop = new(uint)
		*reachedStop = stop
	}

	c.motor.Stop()
	dur := time.Now().Sub(start)
	c.reckon(dur, dir, reachedStop)

	time.Sleep(500 * time.Millisecond)
}

// we need listen so that we can catch switch changes that happen when the motor is off
func (c *Controller) listen() {
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

func (c *Controller) Move(newPosition float64) {
	c.command <- newPosition
	<-c.respond
}

func (c *Controller) Close() {
	c.watcher.Close()
	rpio.Close()
}
