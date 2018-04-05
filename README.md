# go-statusbar
Golang library to create tray apps that live only in OSX's NSStatusBar

## Installation
go-statusbar requires OS X.

`go get github.com/caseymrm/go-statusbar`

## Hello World

```go
package main

import (
	"time"

	"github.com/caseymrm/go-statusbar/tray"
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
```


## License

MIT
