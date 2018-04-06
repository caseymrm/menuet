package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/caseymrm/go-statusbar/tray"
)

var processRe = regexp.MustCompile(`pid (\d+)\(([^)]+)\): \[0x[0-9a-f]+\] (\d\d:\d\d:\d\d) (\w+) named: "(.+)"`)
var sleepKeywords = map[string]bool{
	"PreventUserIdleDisplaySleep": true,
	//"PreventUserIdleSystemSleep":  true,
}
var canSleepTitle = "😴"
var cantSleepTitle = "😫"

func pmset() *tray.MenuState {
	out, err := exec.Command("/usr/bin/pmset", "-g", "assertions").Output()
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	if scanner.Scan() {
		//fmt.Printf("Timestamp: %s\n", scanner.Text())
	}
	if scanner.Scan() {
		//fmt.Printf("Assertion status: %s\n", scanner.Text())
	}
	canSleep := true
	for scanner.Scan() {
		words := strings.Fields(scanner.Text())
		if len(words) != 2 {
			//fmt.Printf("Owning process: %s\n", scanner.Text())
			break
		}
		if words[1] == "1" && sleepKeywords[words[0]] {
			canSleep = false
		}
	}
	ms := tray.MenuState{
		Items: make([]tray.MenuItem, 0, 1),
	}
	for scanner.Scan() {
		matches := processRe.FindSubmatch(scanner.Bytes())
		if len(matches) != 6 {
			continue
		}
		if sleepKeywords[string(matches[4])] {
			ms.Items = append(ms.Items, tray.MenuItem{
				Text:     fmt.Sprintf("%s (pid %s)", matches[5], matches[1]),
				Callback: string(matches[1]),
			})
		}
	}
	if canSleep {
		ms.Title = canSleepTitle
	} else {
		ms.Title = cantSleepTitle
	}
	preAmble := []tray.MenuItem{{Text: "Your laptop can sleep!"}}
	if !canSleep {
		if len(ms.Items) == 1 {
			preAmble = []tray.MenuItem{{Text: "1 process is keeping your laptop awake:"}}
		} else {
			preAmble = []tray.MenuItem{{Text: fmt.Sprintf("%d processes are keeping your laptop awake:", len(ms.Items))}}
		}
	}
	if len(ms.Items) > 0 {
		preAmble = append(preAmble, tray.MenuItem{Text: "---"})
	}
	ms.Items = append(preAmble, ms.Items...)
	return &ms
}

func monitorPmSet() {
	for {
		tray.App().SetMenuState(pmset())
		time.Sleep(10 * time.Second)
	}
}

func handleClicks(callback chan string) {
	for pid := range callback {
		fmt.Printf("PID Clicked %s\n", pid)
	}
}

func main() {
	go monitorPmSet()
	callback := make(chan string)
	app := tray.App()
	app.Clicked = callback
	app.MenuOpened = func() []tray.MenuItem {
		ms := pmset()
		return ms.Items
	}
	go handleClicks(callback)
	app.RunApplication()
}
