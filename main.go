package main

import (
	"fmt"
	"time"

	"github.com/cpucycle/astrotime"
)

const sflat = float64(37.7833)
const sflon = float64(122.4167)

func controlCurtains(c *Curtains) {
	for {
		now := time.Now()
		sunrise := astrotime.NextSunrise(now, sflat, sflon)
		sunset := astrotime.NextSunset(now, sflat, sflon)

		if sunrise.Before(sunset) {
			fmt.Printf("sleeping until %s for sunrise\n", sunrise)
			time.Sleep(sunrise.Sub(now))
			c.Move(1)
			continue
		}

		fmt.Printf("sleeping until %s for sunset\n", sunset)
		time.Sleep(sunset.Sub(now))
		c.Move(0)
	}
}

func main() {
	c := NewCurtains()
	defer c.Close()

	controlCurtains(c)
}
