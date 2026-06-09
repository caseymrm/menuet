# Menuet
Golang library to create menubar apps- programs that live only in OSX's NSStatusBar

## Development Status

Under active development. API still changing rapidly.

## Installation
menuet requires OS X.

`go get github.com/caseymrm/menuet/v2`

## Documentation

https://godoc.org/github.com/caseymrm/menuet

## Left-click handler

Set `Application.Clicked` to intercept left clicks on the menubar icon
instead of opening the menu. The menu still opens on right click (or
Ctrl-left-click). Useful for toggle-style apps — mute audio, pause a
timer, etc. — where the menu is the secondary UI:

```go
menuet.App().Clicked = func() {
    // toggle whatever state your app exposes
}
```

Leave `Clicked` as `nil` (the default) for the standard behavior where
any click opens the menu. Safe to set or clear at runtime; the next
click reflects the current value.

## Running as a real macOS app

`go run` is fine for early development, but several menuet features only work
when the binary is launched from inside a proper macOS `.app` bundle. These
requirements are enforced by macOS, not by menuet:

* **Notifications** require a bundle. `UNUserNotificationCenter` will silently
  no-op for a loose executable. The app also needs to be code-signed —
  ad-hoc signing is not enough; you need a Developer ID signature (or full
  notarization for distribution).
* **Start at Login** writes a launchd plist that points at the executable
  path. You'll typically want that path to be the binary inside your `.app`
  bundle, not a `go run` temp directory.
* **Auto-update** moves a new `.app` bundle on top of the running one, so it
  obviously needs a bundle to update.

The shared `menuet.mk` Makefile assembles a minimal bundle for you. From your
app directory, create a `Makefile` like:

```make
APP=My App
IDENTIFIER=com.example.myapp
include $(GOPATH)/src/github.com/caseymrm/menuet/menuet.mk
```

Then `make run` builds the binary into `My App.app/Contents/MacOS/myapp`,
generates `My App.app/Contents/Info.plist` with your `CFBundleIdentifier`,
and launches it. The `cmd/catalog` example uses this pattern.

To sign the bundle for notifications and distribution, set `IDENTITY` to a
Developer ID Application certificate from your Keychain and run `make sign`.

## Apps built with Menuet

* [Why Awake?](https://github.com/caseymrm/whyawake) - shows why your Mac can't sleep, and lets you force it awake

<img src="https://github.com/caseymrm/whyawake/raw/master/static/cansleep.png" width="25%"/> <img src="https://github.com/caseymrm/whyawake/raw/master/static/cantsleep.png" width="25%"/> <img src="https://github.com/caseymrm/whyawake/raw/master/static/prevented.png" width="25%"/>


* [Not a Fan](https://github.com/caseymrm/notafan) - shows your Mac's temperature and fan speed, notifies you when your CPU is being throttled due to excessive heat

<img src="https://github.com/caseymrm/notafan/raw/master/notafan.png" width="25%"/> <img src="https://github.com/caseymrm/notafan/raw/master/throttled.png" width="25%"/> <img src="https://github.com/caseymrm/notafan/raw/master/notthrottled.png" width="25%"/>

* [Traytter](https://github.com/caseymrm/traytter) - minimalist Twitter client for following a few users

<img src="https://github.com/caseymrm/traytter/raw/master/traytter.png" width="50%"/>

* [Hacker News Menuet](https://github.com/unkrich/hackernews-menuet) - easily browse latest Hacker News posts

<img src="https://github.com/unkrich/hackernews-menuet/blob/master/static/screenshot.png" width="50%"/>

## [Hello World](https://github.com/caseymrm/menuet/tree/master/cmd/helloworld)

```go
package main

import (
	"time"

	"github.com/caseymrm/menuet/v2"
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

```

![Output](https://github.com/caseymrm/menuet/raw/master/static/helloworld.gif)

## Menu items

`MenuItem` is an interface; the concrete types are `Regular` (a normal row)
and `Separator` (a horizontal divider). Construct a menu by returning a
`[]menuet.MenuItem` containing whichever concrete types you need:

```go
menuet.App().Children = func() []menuet.MenuItem {
    return []menuet.MenuItem{
        menuet.Regular{Text: "Status: Active"},
        menuet.Separator{},
        menuet.Regular{Text: "Refresh", Clicked: refresh},
        menuet.Regular{Text: "Submenu", Children: subItems},
    }
}
```

`Regular` carries the familiar fields — `Text`, `Image`, `FontSize`,
`FontWeight`, `State`, `Clicked`, `Children`. Setting `Clicked` makes it
clickable; setting `Children` makes it a submenu.

## Toggle-style apps

For apps where the primary action is a toggle (mute audio, pause a
timer, hide notifications…), macOS menus dismiss the moment the user
clicks an item — there's no public API to "click without closing." Two
patterns work around it:

**Left click toggles, right click opens the menu.** Set
`Application.Clicked` to a callback; left clicks fire the callback
without opening the menu, right clicks (and Ctrl-left-clicks) still
open the menu for secondary actions:

```go
menuet.App().Clicked = func() { toggleMuted() }
```

**Stateful menu items with checkmarks.** For toggles you do want inside
the menu, set `MenuItem.State = true` to show a checkmark, and update
your app state from the `Clicked` callback. The menu will dismiss on
click as usual (OS standard); on the next open, return the items with
the new `State`:

```go
menuet.Regular{
    Text:    "Notifications enabled",
    State:   prefs.NotificationsEnabled,
    Clicked: func() { prefs.NotificationsEnabled = !prefs.NotificationsEnabled },
}
```

## [Catalog](https://github.com/caseymrm/menuet/tree/master/cmd/catalog)

The catalog app is useful for trying many of the possible combinations of features.

<img src="https://github.com/caseymrm/menuet/raw/master/static/catalog.png" width="50%"/>

## Advanced Features

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/caseymrm/menuet/v2"
)

func temperature(woeid string) (temp, unit, text string) {
	url := "https://query.yahooapis.com/v1/public/yql?format=json&q=select%20item.condition%20from%20weather.forecast%20where%20woeid%20%3D%20" + woeid
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	var response struct {
		Query struct {
			Results struct {
				Channel struct {
					Item struct {
						Condition struct {
							Temp string `json:"temp"`
							Text string `json:"text"`
						} `json:"condition"`
					} `json:"item"`
					Units struct {
						Temperature string `json:"temperature"`
					} `json:"units"`
				} `json:"channel"`
			} `json:"results"`
		} `json:"query"`
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&response)
	if err != nil {
		log.Fatal(err)
	}
	return response.Query.Results.Channel.Item.Condition.Temp, response.Query.Results.Channel.Units.Temperature, response.Query.Results.Channel.Item.Condition.Text
}

func location(query string) (string, string) {
	url := "https://query.yahooapis.com/v1/public/yql?format=json&q=select%20woeid,name%20from%20geo.places%20where%20text%3D%22" + url.QueryEscape(query) + "%22"
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Get: %v", err)
		menuet.App().Alert(menuet.Alert{
			MessageText:     "Could not get the weather",
			InformativeText: err.Error(),
		})
		return "", ""
	}
	var response struct {
		Query struct {
			Results struct {
				Place struct {
					Name  string `json:"name"`
					WoeID string `json:"woeid"`
				} `json:"place"`
			} `json:"results"`
		} `json:"query"`
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&response)
	if err != nil {
		log.Printf("Decode: %v", err)
		menuet.App().Alert(menuet.Alert{
			MessageText:     "Could not search for location",
			InformativeText: err.Error(),
		})
		return "", ""
	}
	return response.Query.Results.Place.Name, response.Query.Results.Place.WoeID
}

func temperatureString(woeid string) string {
	temp, unit, text := temperature(woeid)
	return fmt.Sprintf("%s°%s and %s", temp, unit, text)
}

func setWeather() {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: temperatureString(menuet.Defaults().String("loc")),
	})
}

var woeids = map[int]string{
	2442047: "Los Angeles",
	2487956: "San Francisco",
	2459115: "New York",
}

func menuPreview(woeid string) func() []menuet.MenuItem {
	return func() []menuet.MenuItem {
		return []menuet.MenuItem{
			menuet.Regular{
				Text: temperatureString(woeid),
				Clicked: func() {
					setLocation(woeid)
				},
			},
		}
	}
}

func menuItems() []menuet.MenuItem {
	items := []menuet.MenuItem{}

	currentWoeid := menuet.Defaults().String("loc")
	currentNumber, err := strconv.Atoi(currentWoeid)
	if err != nil {
		log.Printf("Atoi: %v", err)
	}
	found := false
	for woeid, name := range woeids {
		woeStr := strconv.Itoa(woeid)
		items = append(items, menuet.Regular{
			Text: name,
			Clicked: func() {
				setLocation(woeStr)
			},
			State:    woeStr == menuet.Defaults().String("loc"),
			Children: menuPreview(woeStr),
		})
		if woeid == currentNumber {
			found = true
		}
	}
	if !found {
		items = append(items, menuet.Regular{
			Text: menuet.Defaults().String("name"),
			Clicked: func() {
				setLocation(currentWoeid)
			},
			Children: menuPreview(currentWoeid),
			State:    true,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].(menuet.Regular).Text < items[j].(menuet.Regular).Text
	})
	items = append(items, menuet.Regular{
		Text: "Other...",
		Clicked: func() {
			response := menuet.App().Alert(menuet.Alert{
				MessageText: "Where would you like to display the weather for?",
				Inputs:      []menuet.AlertInput{{Placeholder: "Location"}},
				Buttons:     []string{"Search", "Cancel"},
			})
			if response.Button == 0 && len(response.Inputs) == 1 && response.Inputs[0] != "" {
				newName, newWoeid := location(response.Inputs[0])
				if newWoeid != "" && newName != "" {
					menuet.Defaults().SetString("loc", newWoeid)
					menuet.Defaults().SetString("name", newName)
					menuet.App().Notification(menuet.Notification{
						Title:    fmt.Sprintf("Showing weather for %s", newName),
						Subtitle: temperatureString(newWoeid),
					})
					setWeather()
				}
			}
		},
	})
	return items
}

func hourlyWeather() {
	for {
		setWeather()
		time.Sleep(time.Hour)
	}
}

func setLocation(woeid string) {
	menuet.Defaults().SetString("loc", woeid)
	setWeather()
}

func main() {
	// Load the location from last time
	woeid := menuet.Defaults().String("loc")
	if woeid == "" {
		menuet.Defaults().SetString("loc", "2442047")
	}

	// Start the hourly check, and set the first value
	go hourlyWeather()

	// Configure the application
	menuet.App().Label = "com.github.caseymrm.menuet.weather"

	// Hook up the on-click to populate the menu
	menuet.App().Children = menuItems

	// Run the app (does not return)
	menuet.App().RunApplication()
}
```

![Output](https://github.com/caseymrm/menuet/raw/master/static/weather.png)

## License

Menuet is licensed under the MIT license, so you are welcome to make closed source menubar apps with it as long as you preserve the copyright. For details see [the LICENSE file](LICENSE).
