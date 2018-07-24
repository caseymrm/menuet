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
		menuet.App().MenuChanged()
		time.Sleep(time.Second)
	}
}

func main() {
	go helloClock()
	menuet.App().MenuOpened = func(key string) []menuet.MenuItem {
		return []menuet.MenuItem{
			{
				Text:     time.Now().Format(":05"),
				Children: true,
			},
		}
	}
	menuet.App().RunApplication()
}
