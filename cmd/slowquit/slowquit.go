// SlowQuit is an example application that demonstrates how to use the
// graceful shutdown handles of a menuet app.
package main

import (
	"log"
	"time"

	"github.com/caseymrm/menuet"
)

func main() {

	// Configure the application
	menuet.App().Label = "com.github.caseymrm.menuet.slowquit"
	menuet.App().HideStartup()
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: "SlowQuit",
	})

	// Hook up the graceful shutdown handles
	wg, ctx := menuet.App().GracefulShutdownHandles()

	// Start our long running routines (e.g an application or http server)
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				log.Println("caught a tick!")
			case <-ctx.Done():
				return
			}

		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Println("caught shutdown signal")
		time.Sleep(3 * time.Second)
		log.Println("bye")
	}()

	// Run the app (does not return)
	menuet.App().RunApplication()
}
