package main

import (
	"fmt"
	"strings"

	"github.com/caseymrm/menuet/v2"
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
		menuet.Regular{
			Text:     "Show Alert",
			Children: alerts,
		},
		menuet.Regular{
			Text:     "Send Notification",
			Children: notifs,
		},
		menuet.Regular{
			Text:     "Change Title",
			Children: changeTitle,
		},
		menuet.Regular{
			Text:     "Menu Items",
			Children: items,
		},
		menuet.Regular{
			Text:     "Left-click handler",
			Children: clickHandlerMenu,
		},
		menuet.Regular{
			Text:     "Search",
			Children: searchDemo,
		},
	}
}

// searchDemo shows a submenu containing a Search field over a static
// list of US states. The Results callback runs on every keystroke; for
// this demo it does a simple case-insensitive substring match, but the
// signature gives you the query string so a real app can do HTTP
// lookups, fuzzy matching, etc.
var demoStates = []string{
	"Alabama", "Alaska", "Arizona", "Arkansas", "California",
	"Colorado", "Connecticut", "Delaware", "Florida", "Georgia",
	"Hawaii", "Idaho", "Illinois", "Indiana", "Iowa", "Kansas",
	"Kentucky", "Louisiana", "Maine", "Maryland", "Massachusetts",
	"Michigan", "Minnesota", "Mississippi", "Missouri", "Montana",
	"Nebraska", "Nevada", "New Hampshire", "New Jersey", "New Mexico",
	"New York", "North Carolina", "North Dakota", "Ohio", "Oklahoma",
	"Oregon", "Pennsylvania", "Rhode Island", "South Carolina",
	"South Dakota", "Tennessee", "Texas", "Utah", "Vermont",
	"Virginia", "Washington", "West Virginia", "Wisconsin", "Wyoming",
}

func searchDemo() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.Search{
			Placeholder: "Filter US states…",
			Results: func(query string) []menuet.MenuItem {
				q := strings.ToLower(query)
				out := make([]menuet.MenuItem, 0, len(demoStates))
				for _, name := range demoStates {
					if q != "" && !strings.Contains(strings.ToLower(name), q) {
						continue
					}
					state := name
					out = append(out, menuet.Regular{
						Text: state,
						Clicked: func() {
							menuet.App().Alert(menuet.Alert{
								MessageText: "You picked " + state,
							})
						},
					})
					if len(out) >= 20 {
						break
					}
				}
				return out
			},
		},
	}
}

var topLevelClicks int

func handleTopLevelClick() {
	topLevelClicks++
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: fmt.Sprintf("Clicks: %d", topLevelClicks),
	})
}

func clickHandlerMenu() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.Regular{
			Text:  "Enabled (left click counts; right click still opens menu)",
			State: menuet.App().Clicked != nil,
			Clicked: func() {
				if menuet.App().Clicked == nil {
					menuet.App().Clicked = handleTopLevelClick
				} else {
					menuet.App().Clicked = nil
					menuet.App().SetMenuState(&menuet.MenuState{Title: "Catalog"})
				}
			},
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
		alerts = append(alerts, menuet.Regular{
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
		notifs = append(notifs, menuet.Regular{
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
		menuet.Regular{
			Text: "Text only",
			Clicked: func() {
				menuet.App().SetMenuState(&menuet.MenuState{
					Title: "Catalog",
				})
			},
		},
		menuet.Regular{
			Text: "Image only",
			Clicked: func() {
				menuet.App().SetMenuState(&menuet.MenuState{
					Image: "clipboard",
				})
			},
		},
		menuet.Regular{
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
		menuet.Regular{
			Text: "Just text",
		},
		menuet.Regular{
			Text:     "FontSizes",
			Children: fontsizes,
		},
		menuet.Regular{
			Text:     "FontWeights",
			Children: fontweights,
		},
		menuet.Regular{
			Text:  "State = true",
			State: true,
		},
		menuet.Regular{
			Text: "Text and Clicked",
			Clicked: func() {
				menuet.App().Alert(menuet.Alert{
					MessageText: "You clicked the inline function",
				})
			},
		},
		menuet.Regular{
			Text: "Text and Children",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					menuet.Regular{
						Text: "Hello",
					},
					menuet.Regular{
						Text: "Inline",
					},
					menuet.Regular{
						Text: "Children",
					},
				}
			},
		},
		menuet.Regular{
			Text:  "Text, Image, and Clicked",
			Image: "clipboard",
			Clicked: func() {
				menuet.App().Alert(menuet.Alert{
					MessageText: "You clicked the inline function",
				})
			},
		},
		menuet.Regular{
			Text:  "Text, Image, and Children",
			Image: "clipboard",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					menuet.Regular{
						Text: "Hello",
					},
					menuet.Regular{
						Text: "Inline",
					},
					menuet.Regular{
						Text: "Children",
					},
				}
			},
		},
		menuet.Regular{
			Text:  "Image and Text",
			Image: "clipboard",
		},
		menuet.Regular{
			Image: "clipboard",
		},
	}
}
func fontsizes() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.Regular{
			Text:     "FontSize 2",
			FontSize: 2,
		},
		menuet.Regular{
			Text:     "FontSize 4",
			FontSize: 4,
		},
		menuet.Regular{
			Text:     "FontSize 6",
			FontSize: 6,
		},
		menuet.Regular{
			Text:     "FontSize 8",
			FontSize: 8,
		},
		menuet.Regular{
			Text:     "FontSize 10",
			FontSize: 10,
		},
		menuet.Regular{
			Text:     "FontSize 12",
			FontSize: 12,
		},
		menuet.Regular{
			Text:     "FontSize 14",
			FontSize: 14,
		},
		menuet.Regular{
			Text:     "FontSize 16",
			FontSize: 16,
		},
		menuet.Regular{
			Text:     "FontSize 18",
			FontSize: 18,
		},
		menuet.Regular{
			Text:     "FontSize 20",
			FontSize: 20,
		},
		menuet.Regular{
			Text:     "FontSize 22",
			FontSize: 22,
		},
		menuet.Regular{
			Text:     "FontSize 24",
			FontSize: 24,
		},
		menuet.Regular{
			Text:     "FontSize 26",
			FontSize: 26,
		},
	}
}

func fontweights() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.Regular{
			Text:       "WeightUltraLight",
			FontWeight: menuet.WeightUltraLight,
		},
		menuet.Regular{
			Text:       "WeightThin",
			FontWeight: menuet.WeightThin,
		},
		menuet.Regular{
			Text:       "WeightLight",
			FontWeight: menuet.WeightLight,
		},
		menuet.Regular{
			Text:       "WeightRegular",
			FontWeight: menuet.WeightRegular,
		},
		menuet.Regular{
			Text:       "WeightMedium",
			FontWeight: menuet.WeightMedium,
		},
		menuet.Regular{
			Text:       "WeightSemibold",
			FontWeight: menuet.WeightSemibold,
		},
		menuet.Regular{
			Text:       "WeightBold",
			FontWeight: menuet.WeightBold,
		},
		menuet.Regular{
			Text:       "WeightHeavy",
			FontWeight: menuet.WeightHeavy,
		},
		menuet.Regular{
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
