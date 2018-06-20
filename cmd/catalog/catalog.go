package main

import (
	"fmt"
	"log"

	"github.com/caseymrm/menuet"
)

var alertsCatalog = []menuet.Alert{
	menuet.Alert{
		MessageText: "Just MessageText",
	},
	menuet.Alert{
		InformativeText: "Just InformativeText",
	},
	menuet.Alert{
		MessageText:     "MessageText and InformativeText",
		InformativeText: "This is the InformativeText",
	},
	menuet.Alert{
		MessageText: "Message and two buttons",
		Buttons:     []string{"One", "Two"},
	},
	menuet.Alert{
		MessageText: "Message and input",
		Inputs:      []string{"Example input"},
	},
	menuet.Alert{
		MessageText:     "Message, InformativeText, Button, and Input",
		InformativeText: "Example InformativeText",
		Buttons:         []string{"Example button"},
		Inputs:          []string{"Example Input"},
	},
	menuet.Alert{
		MessageText: "Message and two inputs",
		Inputs:      []string{"Input one", "Input two"},
	},
}

var notificationsCatalog = []menuet.Notification{
	menuet.Notification{
		Title: "Just a title",
	},
	menuet.Notification{
		Subtitle: "Just a subtitle",
	},
	menuet.Notification{
		Message: "Just a message",
	},
	menuet.Notification{
		Title:    "Title and subtitle",
		Subtitle: "This is the subtitle",
	},
	menuet.Notification{
		Title:   "Title and message",
		Message: "This is the message",
	},
	menuet.Notification{
		Subtitle: "Subtitle and message (this is the subtitle)",
		Message:  "This is the message",
	},
	menuet.Notification{
		Title:    "Title, subtitle, and message",
		Subtitle: "This is the subtitle",
		Message:  "This is the message",
	},
	menuet.Notification{
		Title:        "Action button",
		Subtitle:     "This is a subtitle",
		ActionButton: "Do an action",
	},
	menuet.Notification{
		Title:       "Close button",
		Subtitle:    "This is a subtitle",
		CloseButton: "Custom close button",
	},
	menuet.Notification{
		Title:               "ResponsePlaceholder ",
		Subtitle:            "This is a subtitle",
		ResponsePlaceholder: "Custom responsePlaceholder",
	},
	menuet.Notification{
		Title:      "Identifier set",
		Identifier: "identified",
	},
	menuet.Notification{
		Title: "Remove from notification center",
		RemoveFromNotificationCenter: true,
	},
}

func menuItems() []menuet.MenuItem {
	alerts := make([]menuet.MenuItem, 0, len(alertsCatalog))
	for ind, alert := range alertsCatalog {
		text := alert.MessageText
		if text == "" {
			text = alert.InformativeText
		}
		alerts = append(alerts, menuet.MenuItem{
			Text:     text,
			Callback: fmt.Sprintf("alert %d", ind),
		})
	}

	notifs := make([]menuet.MenuItem, 0, len(notificationsCatalog))
	for ind, notif := range notificationsCatalog {
		text := notif.Title
		if text == "" {
			text = notif.Subtitle
		}
		if text == "" {
			text = notif.Message
		}
		notifs = append(notifs, menuet.MenuItem{
			Text:     text,
			Callback: fmt.Sprintf("notif %d", ind),
		})
	}

	return []menuet.MenuItem{
		menuet.MenuItem{
			Text:     "Show Alert",
			Children: alerts,
		},
		menuet.MenuItem{
			Text:     "Send Notification",
			Children: notifs,
		},
	}
}

func handleClicks(callback chan string) {
	for click := range callback {
		var index int
		var kind string
		n, err := fmt.Sscan(click, &kind, &index)
		if err != nil {
			log.Printf("Sscanf error: %v", err)
			continue
		}
		if n != 2 {
			continue
		}
		switch kind {
		case "notif":
			menuet.App().Notification(notificationsCatalog[index])
		case "alert":
			menuet.App().Alert(alertsCatalog[index])
		}
	}
}

func main() {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: "Catalog",
		Items: menuItems(),
	})
	menuet.App().Label = "com.github.caseymrm.menuet.catalog"

	clickChannel := make(chan string)
	menuet.App().Clicked = clickChannel
	go handleClicks(clickChannel)

	menuet.App().RunApplication()
}
