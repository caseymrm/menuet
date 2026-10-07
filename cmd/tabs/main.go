// Command tabs is a manual check for the Tabs row: two tabs with different
// row counts, so switching shows whether the open menu resizes in place.
package main

import (
	"fmt"

	"github.com/caseymrm/menuet/v2"
)

var selected int

func main() {
	labels := []string{"Short", "Long"}
	menuet.App().Label = "com.github.caseymrm.menuet.tabs"
	menuet.App().Children = func() []menuet.MenuItem {
		items := []menuet.MenuItem{menuet.Tabs{
			Labels: labels, Selected: selected,
			Changed: func(i int) { selected = i },
		}}
		rows := 2
		if selected == 1 {
			rows = 8
		}
		for i := 0; i < rows; i++ {
			items = append(items, menuet.Regular{Text: fmt.Sprintf("%s row %d", labels[selected], i+1), Clicked: func() {}})
		}
		return items
	}
	menuet.App().SetMenuState(&menuet.MenuState{Title: "Tabs"})
	menuet.App().RunApplication()
}
