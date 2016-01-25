package main

import (
	"container/heap"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
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

type watcherAction int

const (
	watcherAdd watcherAction = iota
	watcherRemove
	watcherClose
)

type watcherCmd struct {
	pin    rpio.Pin
	action watcherAction
}

type watcherNotify struct {
	pin   rpio.Pin
	value uint
}

type FDHeap []uintptr

func (h FDHeap) Len() int { return len(h) }

// Less is actually greater (we want a max heap)
func (h FDHeap) Less(i, j int) bool { return h[i] > h[j] }
func (h FDHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *FDHeap) Push(x interface{}) {
	*h = append(*h, x.(uintptr))
}

func (h *FDHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h FDHeap) FdSet() *syscall.FdSet {
	fdset := &syscall.FdSet{}
	for _, val := range h {
		fdset.Bits[val/64] |= 1 << uint(val) % 64
	}
	return fdset
}

const watcherCmdChanLen = 32
const notifyChanLen = 32

type GpioWatcher struct {
	pins       map[uintptr]rpio.Pin
	files      map[uintptr]*os.File
	fds        FDHeap
	cmdChan    chan watcherCmd
	notifyChan chan watcherNotify
}

func NewGpioWatcher() *GpioWatcher {
	gw := &GpioWatcher{
		pins:       make(map[uintptr]rpio.Pin),
		files:      make(map[uintptr]*os.File),
		fds:        FDHeap{},
		cmdChan:    make(chan watcherCmd, watcherCmdChanLen),
		notifyChan: make(chan watcherNotify, notifyChanLen),
	}
	heap.Init(&gw.fds)
	go gw.watch()
	return gw
}

func (gw *GpioWatcher) notify(fdset *syscall.FdSet) {
	for _, fd := range gw.fds {
		if (fdset.Bits[fd/64] & (1 << uint(fd) % 64)) != 0 {
			file := gw.files[fd]
			file.Seek(0, 0)
			buf := make([]byte, 1)
			_, err := file.Read(buf)
			if err != nil {
				if err == io.EOF {
					gw.removeFd(fd)
					continue
				}
				fmt.Printf("failed to read pinfile, %s", err)
				os.Exit(1)
			}
			msg := watcherNotify{
				pin: gw.pins[fd],
			}
			c := buf[0]
			switch c {
			case '0':
				msg.value = 0
			case '1':
				msg.value = 1
			default:
				fmt.Printf("read inconsistent value in pinfile, %c", c)
				os.Exit(1)
			}
			select {
			case gw.notifyChan <- msg:
			default:
			}
		}
	}
}

func (gw *GpioWatcher) fdSelect() {
	timeval := &syscall.Timeval{
		Sec:  1,
		Usec: 0,
	}
	fdset := gw.fds.FdSet()
	n, err := syscall.Select(int(gw.fds[0]+1), nil, nil, fdset, timeval)
	if err != nil {
		fmt.Printf("failed to call syscall.Select, %s", err)
		os.Exit(1)
	}
	if n != 0 {
		gw.notify(fdset)
	}
}

func (gw *GpioWatcher) addPin(p rpio.Pin) {
	f, err := os.Open(fmt.Sprintf("/sys/class/gpio/gpio%d/value", p))
	if err != nil {
		fmt.Printf("failed to open gpio %d value file for reading\n", p)
		os.Exit(1)
	}
	fd := f.Fd()
	gw.pins[fd] = p
	gw.files[fd] = f
	heap.Push(&gw.fds, fd)
}

func (gw *GpioWatcher) removeFd(fd uintptr) {
	// heap operates on an array index, so search heap for fd
	for index, v := range gw.fds {
		if v == fd {
			heap.Remove(&gw.fds, index)
			break
		}
	}
	f := gw.files[fd]
	f.Close()
	delete(gw.pins, fd)
	delete(gw.files, fd)
}

// removePin is only a wrapper around removeFd
// it finds fd given pin and then calls removeFd
func (gw *GpioWatcher) removePin(p rpio.Pin) {
	// we don't index by pin, so go looking
	for fd, pin := range gw.pins {
		if pin == p {
			// found pin
			gw.removeFd(fd)
			return
		}
	}
}

func (gw *GpioWatcher) doCmd(cmd watcherCmd) (shouldContinue bool) {
	shouldContinue = true
	switch cmd.action {
	case watcherAdd:
		gw.addPin(cmd.pin)
	case watcherRemove:
		gw.removePin(cmd.pin)
	case watcherClose:
		shouldContinue = false
	}
	return shouldContinue
}

func (gw *GpioWatcher) recv() (shouldContinue bool) {
	for {
		select {
		case cmd := <-gw.cmdChan:
			shouldContinue = gw.doCmd(cmd)
			if !shouldContinue {
				return
			}
		default:
			shouldContinue = true
			return
		}
	}
}

func (gw *GpioWatcher) watch() {
	for {
		// first we do a syscall.select with timeout if we have any fds to check
		if len(gw.fds) != 0 {
			gw.fdSelect()
		} else {
			// so that we don't churn when the fdset is empty, sleep as if in select call
			time.Sleep(1 * time.Second)
		}
		if gw.recv() == false {
			return
		}
	}
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

func (gw *GpioWatcher) AddPin(p rpio.Pin) {
	exportGPIO(p)
	gw.cmdChan <- watcherCmd{
		pin:    p,
		action: watcherAdd,
	}
}

func (gw *GpioWatcher) RemovePin(p rpio.Pin) {
	gw.cmdChan <- watcherCmd{
		pin:    p,
		action: watcherRemove,
	}
}

func (gw *GpioWatcher) Watch() (p rpio.Pin, v uint) {
	notification := <-gw.notifyChan
	return notification.pin, notification.value
}

func (gw *GpioWatcher) Close() {
	gw.cmdChan <- watcherCmd{
		pin:    0,
		action: watcherClose,
	}
}

func main() {
	watcher := NewGpioWatcher()
	watcher.AddPin(SwitchLeft)
	watcher.AddPin(SwitchRight)
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
