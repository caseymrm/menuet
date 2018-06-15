package main

import (
	"fmt"
	"log"

	"github.com/caseymrm/menuet"
)

var notifications = []menuet.Notification{
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
	notifs := make([]menuet.MenuItem, 0, len(notifications))
	for ind, notif := range notifications {
		text := notif.Title
		if text == "" {
			text = notif.Subtitle
		}
		if text == "" {
			text = notif.Message
		}
		notifs = append(notifs, menuet.MenuItem{
			Text:     text,
			Callback: fmt.Sprintf("notif-%d", ind),
		})
	}
	return []menuet.MenuItem{
		menuet.MenuItem{
			Text:     "Send Notification",
			Children: notifs,
		},
	}
}

func handleClicks(callback chan string) {
	for click := range callback {
		var index int
		n, err := fmt.Sscanf(click, "notif-%d", &index)
		if err != nil {
			log.Printf("Sscanf error: %v", err)
			continue
		}
		if n == 1 {
			menuet.App().Notification(notifications[index])
			continue
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
