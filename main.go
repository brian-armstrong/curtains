package main

import (
	"container/heap"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
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

type watcherAction int

const (
	watcherAdd watcherAction = iota
	watcherRemove
	watcherClose
)

type watcherCmd struct {
	fd     uintptr
	action watcherAction
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
	sync.RWMutex
	pins       map[rpio.Pin]*os.File
	files      map[uintptr]*os.File
	fds        FDHeap
	cmdChan    chan watcherCmd
	notifyChan chan uintptr
}

func NewGpioWatcher() *GpioWatcher {
	gw := &GpioWatcher{
		pins:       make(map[rpio.Pin]*os.File),
		files:      make(map[uintptr]*os.File),
		fds:        FDHeap{},
		cmdChan:    make(chan watcherCmd, watcherCmdChanLen),
		notifyChan: make(chan uintptr, notifyChanLen),
	}
	heap.Init(&gw.fds)
	go gw.watch()
	return gw
}

func (gw *GpioWatcher) notify(fdset *syscall.FdSet) {
	for _, fd := range gw.fds {
		if (fdset.Bits[fd/64] & (1 << uint(fd) % 64)) != 0 {
			select {
			case gw.notifyChan <- fd:
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
	err := syscall.Select(int(gw.fds[0]+1), fdset, nil, nil, timeval)
	if err != nil {
		fmt.Printf("failed to call syscall.Select, %s", err)
		os.Exit(1)
	}
	gw.notify(fdset)
}

func (gw *GpioWatcher) doCmd(cmd watcherCmd) (shouldContinue bool) {
	shouldContinue = true
	switch cmd.action {
	case watcherAdd:
		heap.Push(&gw.fds, cmd.fd)
	case watcherRemove:
		for index, v := range gw.fds {
			if v == cmd.fd {
				heap.Remove(&gw.fds, index)
				break
			}
		}
	case watcherClose:
		shouldContinue = false
	}
	return shouldContinue
}

func (gw *GpioWatcher) watch() {
	for {
		// first we do a syscall.select with timeout if we have any fds to check
		if len(gw.fds) != 0 {
			gw.fdSelect()
		}
		for {
			select {
			case cmd := <-gw.cmdChan:
				shouldContinue := gw.doCmd(cmd)
				if !shouldContinue {
					return
				}
			default:
				break
			}
		}
	}
}

func (gw *GpioWatcher) addPin(p rpio.Pin, f *os.File) {
	gw.Lock()
	defer gw.Unlock()
	gw.pins[p] = f
	gw.files[f.Fd()] = f
}

func (gw *GpioWatcher) AddPin(p rpio.Pin) {
	exportGPIO(p)
	value, err := os.Open(fmt.Sprintf("/sys/class/gpio/gpio%d/value", p))
	if err != nil {
		fmt.Printf("failed to open gpio %d value file for reading\n", p)
		os.Exit(1)
	}
	gw.addPin(p, value)
	gw.cmdChan <- watcherCmd{
		fd:     value.Fd(),
		action: watcherAdd,
	}
}

func (gw *GpioWatcher) removePin(p rpio.Pin) (fd uintptr) {
	gw.Lock()
	defer gw.Unlock()
	f := gw.pins[p]
	fd = f.Fd()
	delete(gw.pins, p)
	delete(gw.files, fd)
	f.Close()
	return fd
}

func (gw *GpioWatcher) RemovePin(p rpio.Pin) {
	fd := gw.removePin(p)
	gw.cmdChan <- watcherCmd{
		fd:     fd,
		action: watcherRemove,
	}
}

func (gw *GpioWatcher) fetch(fd uintptr) (p rpio.Pin, f *os.File, found bool) {
	gw.RLock()
	defer gw.RUnlock()
	f, found = gw.files[fd]
	if !found {
		return 0, nil, false
	}
	for p, pfile := range gw.pins {
		if pfile == f {
			return p, f, true
		}
	}
	// if we get here, it's an inconsistency
	panic("gpiowatcher found a matching fd in fetch but no pin")
}

func (gw *GpioWatcher) Watch() (p rpio.Pin, v uint) {
	// we run a forever loop so that we can discard events for closed files
	for {
		eventFd := <-gw.notifyChan
		pin, file, found := gw.fetch(eventFd)
		if !found {
			continue
		}
		file.Seek(0, 0)
		buf := make([]byte, 1)
		_, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				gw.removePin(pin)
				continue
			}
			fmt.Printf("failed to read pinfile, %s", err)
			os.Exit(1)
		}
		c := buf[0]
		switch c {
		case '0':
			return pin, 0
		case '1':
			return pin, 1
		default:
			fmt.Printf("read inconsistent value in pinfile, %c", c)
			os.Exit(1)
		}
	}
}

func (gw *GpioWatcher) Close() {
	gw.Lock()
	defer gw.Unlock()
	gw.cmdChan <- watcherCmd{
		fd:     0,
		action: watcherClose,
	}
	for _, f := range gw.files {
		f.Close()
	}
	gw.pins = nil
	gw.files = nil
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
