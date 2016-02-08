package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cpucycle/astrotime"
)

type sunEvent uint

const (
	sunriseEvent sunEvent = iota
	sunsetEvent
)

const sflat = float64(37.7833)
const sflon = float64(122.4167)

func sendEverySunrise(c chan<- sunEvent) {
	for {
		now := time.Now()
		sunrise := astrotime.NextSunrise(now, sflat, sflon)
		log.Printf("next sunrise at %s", sunrise)
		time.Sleep(sunrise.Sub(now))
		c <- sunriseEvent
	}
}

func sendEverySunset(c chan<- sunEvent) {
	for {
		now := time.Now()
		sunset := astrotime.NextSunset(now, sflat, sflon)
		log.Printf("next sunset at %s", sunset)
		time.Sleep(sunset.Sub(now))
		c <- sunsetEvent
	}
}

func doSunCmd(c *Controller, ev sunEvent) {
	switch ev {
	case sunriseEvent:
		c.Move(1)
	case sunsetEvent:
		c.Move(0)
	}
}

type requestEvent uint

const (
	openRequest requestEvent = iota
	closeRequest
)

type requestFunc func(c chan<- requestEvent, w http.ResponseWriter, req *http.Request)

func doOpenRequest(c chan<- requestEvent, w http.ResponseWriter, req *http.Request) {
	c <- openRequest
	io.WriteString(w, "opening\n")
}

func doCloseRequest(c chan<- requestEvent, w http.ResponseWriter, req *http.Request) {
	c <- closeRequest
	io.WriteString(w, "closing\n")
}

func doRequestCmd(c *Controller, ev requestEvent) {
	switch ev {
	case openRequest:
		c.Move(1)
	case closeRequest:
		c.Move(0)
	}
}

func controlCurtains(c *Controller) {
	sun := make(chan sunEvent)
	go sendEverySunrise(sun)
	go sendEverySunset(sun)

	request := make(chan requestEvent)
	closure := func(f requestFunc) func(w http.ResponseWriter, req *http.Request) {
		return func(w http.ResponseWriter, req *http.Request) {
			f(request, w, req)
		}
	}

	http.HandleFunc("/open", closure(doOpenRequest))
	http.HandleFunc("/close", closure(doCloseRequest))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("http listen failed, ", err)
	}

	for {
		select {
		case ev := <-sun:
			doSunCmd(c, ev)
		case ev := <-request:
			doRequestCmd(c, ev)
		}
	}
}

func main() {
	c := NewController()
	defer c.Close()
	controlCurtains(c)
}
