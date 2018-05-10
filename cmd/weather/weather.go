package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/caseymrm/menuet"
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

var currentWoeid = "2442047"
var woeids = map[int]string{
	2442047: "Los Angeles",
	2487956: "San Francisco",
	2459115: "New York",
}

func setWeather() {
	temp, unit, text := temperature(currentWoeid)
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: fmt.Sprintf("%s°%s and %s", temp, unit, text),
	})
}

func hourlyWeather() {
	for {
		setWeather()
		time.Sleep(time.Hour)
	}
}

func handleClicks(callback chan string) {
	for woeid := range callback {
		currentWoeid = woeid
		setWeather()
	}
}

func main() {
	go hourlyWeather()
	clickChannel := make(chan string)
	menuet.App().Label = "com.github.caseymrm.menuet.weather"
	menuet.App().Clicked = clickChannel
	menuet.App().MenuOpened = func() []menuet.MenuItem {
		items := []menuet.MenuItem{}
		for woeid, name := range woeids {
			items = append(items, menuet.MenuItem{
				Text:     name,
				Callback: strconv.Itoa(woeid),
				State:    strconv.Itoa(woeid) == currentWoeid,
			})
		}
		return items
	}
	go handleClicks(clickChannel)
	menuet.App().RunApplication()
}
