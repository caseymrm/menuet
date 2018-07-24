package main

import (
	"time"

	"github.com/caseymrm/menuet"
)

func helloClock() {
	for {
		menuet.App().SetMenuState(&menuet.MenuState{
			Title: "Hello World " + time.Now().Format(":05"),
		})
		time.Sleep(time.Second)
	}
}

func main() {
	go helloClock()
	menuet.App().RunApplication()
}
