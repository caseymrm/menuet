package main

import (
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
		Inputs:      []menuet.AlertInput{{Placeholder: "Example input"}},
	},
	menuet.Alert{
		MessageText:     "Message, InformativeText, Button, and Input",
		InformativeText: "Example InformativeText",
		Buttons:         []string{"Example button"},
		Inputs:          []menuet.AlertInput{{Placeholder: "Example Input"}},
	},
	menuet.Alert{
		MessageText: "Message and two inputs",
		Inputs:      []menuet.AlertInput{{Placeholder: "Input one"}, {Placeholder: "Input two"}},
	},
	menuet.Alert{
		MessageText: "Login form",
		Buttons:     []string{"Login", "Cancel"},
		Inputs: []menuet.AlertInput{
			{Placeholder: "Username"},
			{Placeholder: "Password", Type: menuet.InputPassword},
		},
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
		Title:                        "Remove from notification center",
		RemoveFromNotificationCenter: true,
	},
}

func menuItems() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.MenuItem{
			Text:     "Show Alert",
			Children: alerts,
		},
		menuet.MenuItem{
			Text:     "Send Notification",
			Children: notifs,
		},
		menuet.MenuItem{
			Text:     "Change Title",
			Children: changeTitle,
		},
		menuet.MenuItem{
			Text:     "Menu Items",
			Children: items,
		},
	}
}

func alerts() []menuet.MenuItem {
	alerts := make([]menuet.MenuItem, 0, len(alertsCatalog))
	for _, alert := range alertsCatalog {
		alert := alert
		text := alert.MessageText
		if text == "" {
			text = alert.InformativeText
		}
		alerts = append(alerts, menuet.MenuItem{
			Text: text,
			Clicked: func() {
				menuet.App().Alert(alert)
			},
		})
	}
	return alerts
}

func notifs() []menuet.MenuItem {
	notifs := make([]menuet.MenuItem, 0, len(notificationsCatalog))
	for _, notif := range notificationsCatalog {
		notif := notif
		text := notif.Title
		if text == "" {
			text = notif.Subtitle
		}
		if text == "" {
			text = notif.Message
		}
		notifs = append(notifs, menuet.MenuItem{
			Text: text,
			Clicked: func() {
				menuet.App().Notification(notif)
			},
		})
	}
	return notifs
}

func changeTitle() []menuet.MenuItem {
	return []menuet.MenuItem{
		{
			Text: "Text only",
			Clicked: func() {
				menuet.App().SetMenuState(&menuet.MenuState{
					Title: "Catalog",
				})
			},
		},
		{
			Text: "Image only",
			Clicked: func() {
				menuet.App().SetMenuState(&menuet.MenuState{
					Image: "clipboard",
				})
			},
		},
		{
			Text: "Text and Image",
			Clicked: func() {
				menuet.App().SetMenuState(&menuet.MenuState{
					Title: "Catalog",
					Image: "clipboard",
				})
			},
		},
	}
}

func items() []menuet.MenuItem {
	return []menuet.MenuItem{
		{
			Text: "Just text",
		},
		{
			Text:     "FontSizes",
			Children: fontsizes,
		},
		{
			Text:     "FontWeights",
			Children: fontweights,
		},
		{
			Text:  "State = true",
			State: true,
		},
		{
			Text: "Text and Clicked",
			Clicked: func() {
				menuet.App().Alert(menuet.Alert{
					MessageText: "You clicked the inline function",
				})
			},
		},
		{
			Text: "Text and Children",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					{
						Text: "Hello",
					},
					{
						Text: "Inline",
					},
					{
						Text: "Children",
					},
				}
			},
		},
		{
			Text:  "Text, Image, and Clicked",
			Image: "clipboard",
			Clicked: func() {
				menuet.App().Alert(menuet.Alert{
					MessageText: "You clicked the inline function",
				})
			},
		},
		{
			Text:  "Text, Image, and Children",
			Image: "clipboard",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					{
						Text: "Hello",
					},
					{
						Text: "Inline",
					},
					{
						Text: "Children",
					},
				}
			},
		},
		{
			Text:  "Image and Text",
			Image: "clipboard",
		},
		{
			Image: "clipboard",
		},
	}
}
func fontsizes() []menuet.MenuItem {
	return []menuet.MenuItem{
		{
			Text:     "FontSize 2",
			FontSize: 2,
		},
		{
			Text:     "FontSize 4",
			FontSize: 4,
		},
		{
			Text:     "FontSize 6",
			FontSize: 6,
		},
		{
			Text:     "FontSize 8",
			FontSize: 8,
		},
		{
			Text:     "FontSize 10",
			FontSize: 10,
		},
		{
			Text:     "FontSize 12",
			FontSize: 12,
		},
		{
			Text:     "FontSize 14",
			FontSize: 14,
		},
		{
			Text:     "FontSize 16",
			FontSize: 16,
		},
		{
			Text:     "FontSize 18",
			FontSize: 18,
		},
		{
			Text:     "FontSize 20",
			FontSize: 20,
		},
		{
			Text:     "FontSize 22",
			FontSize: 22,
		},
		{
			Text:     "FontSize 24",
			FontSize: 24,
		},
		{
			Text:     "FontSize 26",
			FontSize: 26,
		},
	}
}

func fontweights() []menuet.MenuItem {
	return []menuet.MenuItem{
		{
			Text:       "WeightUltraLight",
			FontWeight: menuet.WeightUltraLight,
		},
		{
			Text:       "WeightThin",
			FontWeight: menuet.WeightThin,
		},
		{
			Text:       "WeightLight",
			FontWeight: menuet.WeightLight,
		},
		{
			Text:       "WeightRegular",
			FontWeight: menuet.WeightRegular,
		},
		{
			Text:       "WeightMedium",
			FontWeight: menuet.WeightMedium,
		},
		{
			Text:       "WeightSemibold",
			FontWeight: menuet.WeightSemibold,
		},
		{
			Text:       "WeightBold",
			FontWeight: menuet.WeightBold,
		},
		{
			Text:       "WeightHeavy",
			FontWeight: menuet.WeightHeavy,
		},
		{
			Text:       "WeightBlack",
			FontWeight: menuet.WeightBlack,
		},
	}
}

func main() {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: "Catalog",
	})
	menuet.App().Label = "com.github.caseymrm.menuet.catalog"
	menuet.App().Children = menuItems
	menuet.App().RunApplication()
}
