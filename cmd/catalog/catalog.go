package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
		menuet.Regular{
			Text:     "Rich text",
			Children: richTextDemo,
		},
		menuet.Regular{
			Text:     "Hotkeys",
			Children: hotkeysDemo,
		},
		menuet.Regular{
			Text:     "Images",
			Children: imageDemo,
		},
	}
}

// ---------------------------------------------------------------------------
// Images
//
// menuet.Image is a row that draws a picture instead of text. As the only
// child of a Regular it makes a submenu that *is* an image.
//
// The pictures below are generated in Go, which is also the point: menuet
// never fetches an image itself (that would run on the thread building the
// menu, and couldn't carry an auth header). Whatever your app can produce or
// download — an authenticated screenshot, a chart — it hands over as Data or
// a Path.
// ---------------------------------------------------------------------------

var (
	chartOnce sync.Once
	chartData []byte

	shotOnce sync.Once
	shotData []byte
	shotPath string

	imageClicks int
)

func imageDemo() []menuet.MenuItem {
	return []menuet.MenuItem{
		// The headline: the whole submenu is one picture.
		menuet.Regular{
			Text: "Submenu that is an image",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					menuet.Image{Data: chartPNG(), MaxWidth: 400},
				}
			},
		},

		// The shape a real app usually wants: picture, caption, action.
		menuet.Regular{
			Text: "Picture, caption, action",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					menuet.Image{Data: screenshotPNG(), MaxWidth: 480},
					menuet.Regular{Runs: []menuet.TextRun{
						{Text: "Captured ", Color: menuet.LabelSecondary, FontSize: 11},
						{Text: time.Now().Format("3:04:05 PM"),
							Color: menuet.LabelSecondary, FontSize: 11, Monospaced: true},
					}},
					menuet.Separator{},
					menuet.Regular{Text: "Open full size", Clicked: openFullSize},
				}
			},
		},

		// Clicked makes the picture itself a button: it highlights under the
		// pointer and dismisses the menu on click, like any other row.
		menuet.Regular{
			Text: "Clickable image",
			Children: func() []menuet.MenuItem {
				return []menuet.MenuItem{
					menuet.Image{
						Data:     chartPNG(),
						MaxWidth: 320,
						Clicked: func() {
							imageClicks++
							menuet.App().MenuChanged()
						},
					},
					menuet.Regular{Runs: []menuet.TextRun{
						{Text: fmt.Sprintf("Clicked %d time(s) — click the chart",
							imageClicks), Color: menuet.LabelSecondary, FontSize: 11},
					}},
				}
			},
		},

		// Path source. For a big image only the path crosses the cgo bridge,
		// rather than ~1MB of base64 on every single menu open.
		menuet.Regular{
			Text: "From a file path",
			Children: func() []menuet.MenuItem {
				path := screenshotFilePath()
				if path == "" {
					return []menuet.MenuItem{
						menuet.Regular{Text: "couldn't write the temp file"},
					}
				}
				return []menuet.MenuItem{
					menuet.Image{Path: path, MaxWidth: 480},
					menuet.Regular{Runs: []menuet.TextRun{
						{Text: path, Color: menuet.LabelTertiary, FontSize: 10, Monospaced: true},
					}},
				}
			},
		},

		// One 1440x900 source, three bounds. It only ever scales *down* —
		// never up — so the aspect ratio always survives.
		menuet.Regular{
			Text: "Scale to fit (one 1440×900 source)",
			Children: func() []menuet.MenuItem {
				label := func(s string) menuet.MenuItem {
					return menuet.Regular{Runs: []menuet.TextRun{
						{Text: s, Color: menuet.LabelSecondary, FontSize: 11, Monospaced: true},
					}}
				}
				return []menuet.MenuItem{
					label("MaxWidth: 160"),
					menuet.Image{Data: screenshotPNG(), MaxWidth: 160},
					label("MaxWidth: 300"),
					menuet.Image{Data: screenshotPNG(), MaxWidth: 300},
					label("default bound (480×360)"),
					menuet.Image{Data: screenshotPNG()},
				}
			},
		},
	}
}

func openFullSize() {
	if path := screenshotFilePath(); path != "" {
		exec.Command("open", path).Run()
	}
}

// chartPNG renders a small bar chart. Built once: Children runs on every menu
// open, and re-encoding a PNG each time would be wasted work.
func chartPNG() []byte {
	chartOnce.Do(func() {
		const w, h = 400, 160
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		fill(img, img.Bounds(), color.RGBA{0x1c, 0x1c, 0x20, 0xff})
		bars := []int{40, 95, 62, 130, 78, 110, 55}
		barW := w / (len(bars)*2 + 1)
		for i, v := range bars {
			c := color.RGBA{0x0a, 0x84, 0xff, 0xff}
			if v == 130 {
				c = color.RGBA{0x30, 0xd1, 0x58, 0xff} // the peak
			}
			x := barW + i*2*barW
			fill(img, image.Rect(x, h-v-12, x+barW, h-12), c)
		}
		chartData = encodePNG(img)
	})
	return chartData
}

// screenshotPNG renders a 1440x900 stand-in for a real screen capture — big
// enough that the scale-to-fit path is doing real work.
func screenshotPNG() []byte {
	shotOnce.Do(buildScreenshot)
	return shotData
}

// screenshotFilePath is the same picture on disk, for the Path source.
func screenshotFilePath() string {
	shotOnce.Do(buildScreenshot)
	return shotPath
}

func buildScreenshot() {
	const w, h = 1440, 900
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(30 + 90*x/w),
				G: uint8(40 + 60*y/h),
				B: uint8(120 + 100*(w-x)/w),
				A: 0xff,
			})
		}
	}
	// A few "windows", so the downscaled picture still reads as a screen.
	for i, r := range []image.Rectangle{
		image.Rect(80, 90, 700, 520),
		image.Rect(420, 300, 1180, 780),
		image.Rect(900, 80, 1360, 380),
	} {
		shade := uint8(235 - 45*i)
		fill(img, r, color.RGBA{shade, shade, shade, 0xff})
		fill(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+30),
			color.RGBA{0x3a, 0x3a, 0x42, 0xff})
	}
	shotData = encodePNG(img)

	path := filepath.Join(os.TempDir(), "menuet-catalog-screenshot.png")
	if err := os.WriteFile(path, shotData, 0o600); err != nil {
		log.Printf("catalog: writing demo screenshot: %v", err)
		return
	}
	shotPath = path
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Printf("catalog: encoding png: %v", err)
		return nil
	}
	return buf.Bytes()
}

var hotkeyFireCount int

func hotkeysDemo() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.Regular{
			Text:     fmt.Sprintf("Cmd+Shift+M fired %d time(s)", hotkeyFireCount),
			Shortcut: &menuet.Shortcut{KeyCode: menuet.KeyM, Modifiers: menuet.ModCmd | menuet.ModShift},
			Clicked: func() {
				hotkeyFireCount++
				menuet.App().MenuChanged()
			},
		},
		menuet.Regular{
			Text:     "Cmd+Alt+1 → flash an alert",
			Shortcut: &menuet.Shortcut{KeyCode: menuet.Key1, Modifiers: menuet.ModCmd | menuet.ModAlt},
			Clicked: func() {
				menuet.App().Alert(menuet.Alert{MessageText: "Cmd+Alt+1 fired"})
			},
		},
	}
}

func richTextDemo() []menuet.MenuItem {
	return []menuet.MenuItem{
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "🏆 ", FontSize: 16},
				{
					Text:       "LAL WINS",
					FontWeight: menuet.WeightHeavy,
					FontSize:   15,
					Color:      menuet.Color{R: 230, G: 180, B: 30, A: 255},
					Shadow: &menuet.Shadow{
						Color: menuet.Color{R: 255, G: 220, B: 80, A: 220},
						Blur:  8,
					},
				},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Underline", Underline: true},
				{Text: "  ·  "},
				{Text: "Strikethrough", Strikethrough: true, Color: menuet.LabelSecondary},
				{Text: "  ·  "},
				{Text: " marker ", Background: menuet.Color{R: 255, G: 240, B: 130, A: 255}},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Underline color independent of text: "},
				{
					Text:           "misspelled",
					Underline:      true,
					UnderlineColor: menuet.SystemRed,
				},
				{Text: "  /  "},
				{
					Text:               "outdated",
					Color:              menuet.LabelSecondary,
					Strikethrough:      true,
					StrikethroughColor: menuet.SystemRed,
				},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Semantic ", Color: menuet.LabelPrimary},
				{Text: "colors ", Color: menuet.LabelSecondary},
				{Text: "adapt ", Color: menuet.LabelTertiary},
				{Text: "to dark mode", Color: menuet.LabelQuaternary},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Status: ", Color: menuet.LabelSecondary},
				{Text: "FAILED", Color: menuet.SystemRed, FontWeight: menuet.WeightBold},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "GSW 71 – 68 MIN  "},
				{Text: "LIVE", Color: menuet.SystemRed, Badge: true},
			},
			Subtitle: []menuet.TextRun{
				{Text: "NBA · Q3 5:42"},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "$ ", Color: menuet.LabelTertiary, Monospaced: true},
				{Text: "make release", Monospaced: true},
			},
			Subtitle: []menuet.TextRun{
				{Text: "Builds + signs + uploads zip to GitHub"},
			},
		},
		menuet.Regular{
			Text:  "Item-level color (red)",
			Color: menuet.Red,
		},
		menuet.Regular{
			Text:       "Monospaced",
			Monospaced: true,
		},
		menuet.Regular{
			Text:       "Mono + bold",
			Monospaced: true,
			FontWeight: menuet.WeightBold,
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Status: "},
				{Text: "FAILED", Color: menuet.Red, FontWeight: menuet.WeightBold},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Status: "},
				{Text: "OK", Color: menuet.Green, FontWeight: menuet.WeightBold},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "Build "},
				{Text: "#1234 ", Color: menuet.Gray, Monospaced: true},
				{Text: "passed in "},
				{Text: "42.3s", FontWeight: menuet.WeightSemibold},
			},
		},
		menuet.Regular{
			Runs: []menuet.TextRun{
				{Text: "$ ", Color: menuet.Gray, Monospaced: true},
				{Text: "make release", Monospaced: true},
			},
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
		menuet.Regular{
			Text: "Runs in the title (live score)",
			Clicked: func() {
				menuet.App().SetMenuState(&menuet.MenuState{
					Runs: []menuet.TextRun{
						{Text: "● ", Color: menuet.SystemRed},
						{Text: "GSW ", FontWeight: menuet.WeightSemibold},
						{Text: "71", FontWeight: menuet.WeightBold, Monospaced: true},
						{Text: "–68", Color: menuet.LabelSecondary, Monospaced: true},
						{Text: " MIN", Color: menuet.LabelSecondary},
					},
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
