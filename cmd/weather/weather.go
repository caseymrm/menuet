package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/caseymrm/go-statusbar/tray"
)

func temperature(woeid string) int {
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
							Temp int `json:"temp,string"`
						} `json:"condition"`
					} `json:"item"`
				} `json:"channel"`
			} `json:"results"`
		} `json:"query"`
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&response)
	if err != nil {
		log.Fatal(err)
	}
	return response.Query.Results.Channel.Item.Condition.Temp
}

func hourlyWeather() {
	for {
		laWeather := temperature("2442047")
		laStr := strconv.Itoa(laWeather)
		tray.App().SetMenuState(&tray.MenuState{
			Title: laStr + "°",
		})
		time.Sleep(time.Hour)
	}
}
func main() {
	go hourlyWeather()
	tray.App().RunApplication()
}
