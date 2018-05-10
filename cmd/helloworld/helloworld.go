package main

import (
	"time"

	"github.com/caseymrm/menuet/tray"
)

func helloClock() {
	for {
		tray.App().SetMenuState(&tray.MenuState{
			Title: "Hello World " + time.Now().Format(":05"),
		})
		time.Sleep(time.Second)
	}
}
func main() {
	go helloClock()
	tray.App().RunApplication()
}
