package main

import (
	"fmt"

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

func menuItems(item menuet.MenuItem) []menuet.MenuItem {
	if item.Type == menuet.Root {
		return []menuet.MenuItem{
			menuet.MenuItem{
				Text:     "Show Alert",
				Data:     "alerts",
				Children: true,
			},
			menuet.MenuItem{
				Text:     "Send Notification",
				Data:     "notifs",
				Children: true,
			},
			menuet.MenuItem{
				Text:     "Menu Items",
				Data:     "items",
				Children: true,
			},
		}
	}
	switch item.Data {
	case "alerts":
		alerts := make([]menuet.MenuItem, 0, len(alertsCatalog))
		for ind, alert := range alertsCatalog {
			text := alert.MessageText
			if text == "" {
				text = alert.InformativeText
			}
			alerts = append(alerts, menuet.MenuItem{
				Text: text,
				Data: fmt.Sprintf("alert %d", ind),
			})
		}
		return alerts
	case "notifs":
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
				Text: text,
				Data: fmt.Sprintf("notif %d", ind),
			})
		}
		return notifs
	case "items":
		return []menuet.MenuItem{
			{
				Text: "Text no key",
			},
			{
				Text: "Text with key",
				Data: "Text with key",
			},
			{
				Text:     "Text with key, disabled",
				Data:     "Text with key, disabled",
				Disabled: true,
			},
			{
				Text:     "FontSizes",
				Data:     "fontsizes",
				Children: true,
			},
			{
				Text:     "FontWeights",
				Data:     "fontweights",
				Children: true,
			},
			{
				Text:  "State = true",
				Data:  "State = true",
				State: true,
			},
			{
				Text: "Text and inline Clicked",
				Clicked: func() {
					menuet.App().Alert(menuet.Alert{
						MessageText: "Just MessageText",
					})
				},
			},
			{
				Text: "Text and inline MenuOpened",
				MenuOpened: func() []menuet.MenuItem {
					return []menuet.MenuItem{
						{
							Text: "Hello",
						},
					}
				},
			},
		}
	case "fontsizes":
		return []menuet.MenuItem{
			{
				Text:     "FontSize 2",
				Data:     "FontSize 2",
				FontSize: 2,
			},
			{
				Text:     "FontSize 4",
				Data:     "FontSize 4",
				FontSize: 4,
			},
			{
				Text:     "FontSize 6",
				Data:     "FontSize 6",
				FontSize: 6,
			},
			{
				Text:     "FontSize 8",
				Data:     "FontSize 8",
				FontSize: 8,
			},
			{
				Text:     "FontSize 10",
				Data:     "FontSize 10",
				FontSize: 10,
			},
			{
				Text:     "FontSize 12",
				Data:     "FontSize 12",
				FontSize: 12,
			},
			{
				Text:     "FontSize 14",
				Data:     "FontSize 14",
				FontSize: 14,
			},
			{
				Text:     "FontSize 16",
				Data:     "FontSize 16",
				FontSize: 16,
			},
			{
				Text:     "FontSize 18",
				Data:     "FontSize 18",
				FontSize: 18,
			},
			{
				Text:     "FontSize 20",
				Data:     "FontSize 20",
				FontSize: 20,
			},
			{
				Text:     "FontSize 22",
				Data:     "FontSize 22",
				FontSize: 22,
			},
			{
				Text:     "FontSize 24",
				Data:     "FontSize 24",
				FontSize: 24,
			},
			{
				Text:     "FontSize 26",
				Data:     "FontSize 26",
				FontSize: 26,
			},
		}
	case "fontweights":
		return []menuet.MenuItem{
			{
				Text:       "WeightUltraLight",
				FontWeight: menuet.WeightUltraLight,
				Data:       "WeightUltraLight",
			},
			{
				Text:       "WeightThin",
				FontWeight: menuet.WeightThin,
				Data:       "WeightThin",
			},
			{
				Text:       "WeightLight",
				FontWeight: menuet.WeightLight,
				Data:       "WeightLight",
			},
			{
				Text:       "WeightRegular",
				FontWeight: menuet.WeightRegular,
				Data:       "WeightRegular",
			},
			{
				Text:       "WeightMedium",
				FontWeight: menuet.WeightMedium,
				Data:       "WeightMedium",
			},
			{
				Text:       "WeightSemibold",
				FontWeight: menuet.WeightSemibold,
				Data:       "WeightSemibold",
			},
			{
				Text:       "WeightBold",
				FontWeight: menuet.WeightBold,
				Data:       "WeightBold",
			},
			{
				Text:       "WeightHeavy",
				FontWeight: menuet.WeightHeavy,
				Data:       "WeightHeavy",
			},
			{
				Text:       "WeightBlack",
				FontWeight: menuet.WeightBlack,
				Data:       "WeightBlack",
			},
		}
	}
	return nil
}

func handleClick(item menuet.MenuItem) {
	click, ok := item.Data.(string)
	if !ok {
		return
	}
	var index int
	var kind string
	n, err := fmt.Sscan(click, &kind, &index)
	if err != nil {
		return
	}
	if n != 2 {
		return
	}
	switch kind {
	case "notif":
		menuet.App().Notification(notificationsCatalog[index])
	case "alert":
		menuet.App().Alert(alertsCatalog[index])
	}
}

func main() {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: "Catalog",
	})
	menuet.App().Label = "com.github.caseymrm.menuet.catalog"
	menuet.App().Clicked = handleClick
	menuet.App().MenuOpened = menuItems
	menuet.App().RunApplication()
}
