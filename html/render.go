// Package html renders a menuet.Snapshot to a self-contained HTML
// fragment that approximates how the menu looks in the macOS menu bar.
//
// The renderer is intentionally a pure data → HTML transform: it never
// runs code from the snapshot, never fetches remote resources, and only
// emits a fixed set of tags and CSS properties. That makes it safe to
// run on snapshots from untrusted third-party apps — anything outside
// the known schema is either ignored or rendered as escaped text.
//
// Output shape: a single <div class="menumock-…"> tree with inline styles
// using a fixed set of CSS custom properties (--bar-bg, --panel-bg,
// --text, etc.) set by the caller. The default theme is light; pass
// Options{Theme: ThemeDark} for dark.
package html

import (
	"fmt"
	stdhtml "html"
	"strings"

	"github.com/caseymrm/menuet/v2"
)

// Theme picks the color palette. Both themes match the look from the
// menuet.app design — translucent panel, subtle border, text hierarchy.
type Theme int

const (
	ThemeLight Theme = iota
	ThemeDark
)

// Options tweaks rendering. Zero value renders a light-theme mockup.
type Options struct {
	Theme Theme

	// AccentHex overrides the system accent color used for selection
	// highlights and the menu-bar title pill (when the snapshot's
	// State.Runs uses the accent semantic color). Empty falls back to
	// the theme default (#007aff light / #0a84ff dark). Must be a
	// 7-char "#rrggbb" string; other values are dropped.
	AccentHex string
}

// Render produces an HTML fragment for the snapshot using the given
// options. The output is safe to inline into a larger page: it brings
// its own CSS-variable scope and only emits whitelisted tags/attrs/
// CSS properties.
//
// If snap.Schema is not the version this renderer was built for, Render
// returns an HTML comment noting the mismatch and the rest of the snap
// is still rendered best-effort. Future schema bumps will gate stricter
// behavior off the version string.
func Render(snap menuet.Snapshot, opts Options) string {
	var b strings.Builder
	b.WriteString(`<div class="menumock" style="`)
	b.WriteString(themeVars(opts))
	b.WriteString(`">`)
	if snap.Schema != "" && snap.Schema != menuet.SnapshotSchema {
		fmt.Fprintf(&b, `<!-- menuet/html: unknown schema %q (expected %q) -->`,
			snap.Schema, menuet.SnapshotSchema)
	}
	b.WriteString(`<div style="font-family: -apple-system, BlinkMacSystemFont, system-ui, 'SF Pro Text', sans-serif; width: 332px; display: flex; flex-direction: column; align-items: flex-end; gap: 7px; -webkit-font-smoothing: antialiased;">`)
	renderBar(&b, snap.State)
	renderPanel(&b, snap.Items)
	b.WriteString(`</div></div>`)
	return b.String()
}

func themeVars(opts Options) string {
	accent := opts.AccentHex
	if !isHexColor(accent) {
		accent = ""
	}
	if opts.Theme == ThemeDark {
		if accent == "" {
			accent = "#0a84ff"
		}
		return joinVars(map[string]string{
			"--bar-bg":       "rgba(28,28,30,0.46)",
			"--bar-text":     "rgba(255,255,255,0.9)",
			"--panel-bg":     "rgba(46,46,48,0.74)",
			"--panel-border": "rgba(255,255,255,0.14)",
			"--shadow":       "0 20px 54px rgba(0,0,0,0.55), 0 3px 12px rgba(0,0,0,0.4)",
			"--text":         "rgba(255,255,255,0.92)",
			"--text-2":       "rgba(255,255,255,0.52)",
			"--text-3":       "rgba(255,255,255,0.36)",
			"--sep":          "rgba(255,255,255,0.12)",
			"--field-bg":     "rgba(255,255,255,0.10)",
			"--hover":        "rgba(255,255,255,0.10)",
			"--accent":       accent,
			"--live":         "#ff453a",
		})
	}
	if accent == "" {
		accent = "#007aff"
	}
	return joinVars(map[string]string{
		"--bar-bg":       "rgba(255,255,255,0.42)",
		"--bar-text":     "rgba(0,0,0,0.82)",
		"--panel-bg":     "rgba(249,249,250,0.78)",
		"--panel-border": "rgba(0,0,0,0.10)",
		"--shadow":       "0 16px 44px rgba(0,0,0,0.22), 0 3px 10px rgba(0,0,0,0.10)",
		"--text":         "rgba(0,0,0,0.86)",
		"--text-2":       "rgba(0,0,0,0.48)",
		"--text-3":       "rgba(0,0,0,0.34)",
		"--sep":          "rgba(0,0,0,0.10)",
		"--field-bg":     "rgba(0,0,0,0.06)",
		"--hover":        "rgba(0,0,0,0.07)",
		"--accent":       accent,
		"--live":         "#ff3b30",
	})
}

func joinVars(m map[string]string) string {
	// Stable order so identical inputs produce identical output.
	keys := []string{
		"--bar-bg", "--bar-text", "--panel-bg", "--panel-border", "--shadow",
		"--text", "--text-2", "--text-3", "--sep", "--field-bg", "--hover",
		"--accent", "--live",
	}
	var b strings.Builder
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		b.WriteString(k)
		b.WriteString(":")
		b.WriteString(v)
		b.WriteString(";")
	}
	return b.String()
}

func renderBar(b *strings.Builder, state *menuet.MenuState) {
	b.WriteString(`<div style="width: 100%; display: flex; justify-content: flex-end; align-items: center; gap: 13px; padding: 0 8px; height: 24px; background: var(--bar-bg); -webkit-backdrop-filter: blur(20px) saturate(1.4); backdrop-filter: blur(20px) saturate(1.4); border-radius: 7px; color: var(--bar-text); font-size: 12.5px;">`)
	if state != nil {
		switch {
		case len(state.Runs) > 0:
			b.WriteString(`<div style="display: flex; align-items: center; gap: 4px; padding: 2px 7px; border-radius: 5px;">`)
			for _, run := range state.Runs {
				renderRun(b, run, true)
			}
			b.WriteString(`</div>`)
		case state.Title != "":
			b.WriteString(`<span style="font-weight: 600;">`)
			b.WriteString(stdhtml.EscapeString(state.Title))
			b.WriteString(`</span>`)
		}
	}
	b.WriteString(`<span style="opacity: 0.6; font-variant-numeric: tabular-nums;">Wed 9:41 AM</span>`)
	b.WriteString(`</div>`)
}

func renderPanel(b *strings.Builder, items []menuet.SnapshotItem) {
	b.WriteString(`<div style="width: 320px; background: var(--panel-bg); -webkit-backdrop-filter: blur(34px) saturate(1.7); backdrop-filter: blur(34px) saturate(1.7); border: 0.5px solid var(--panel-border); border-radius: 11px; box-shadow: var(--shadow); padding: 5px 0;">`)
	for _, item := range items {
		renderItem(b, item)
	}
	b.WriteString(`</div>`)
}

func renderItem(b *strings.Builder, item menuet.SnapshotItem) {
	switch item.Type {
	case "separator":
		b.WriteString(`<div style="height: 1px; background: var(--sep); margin: 5px 12px;"></div>`)
	case "search":
		b.WriteString(`<div style="margin: 1px 8px 5px; display: flex; align-items: center; gap: 6px; height: 27px; padding: 0 9px; background: var(--field-bg); border-radius: 6px; color: var(--text-3); font-size: 13px;">`)
		b.WriteString(`<svg width="13" height="13" viewBox="0 0 13 13" fill="none" style="flex: none;"><circle cx="5.4" cy="5.4" r="3.9" stroke="currentColor" stroke-width="1.3"></circle><line x1="8.4" y1="8.4" x2="11.3" y2="11.3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"></line></svg>`)
		placeholder := item.Text
		if placeholder == "" {
			placeholder = "Search…"
		}
		b.WriteString(`<span>`)
		b.WriteString(stdhtml.EscapeString(placeholder))
		b.WriteString(`</span>`)
		b.WriteString(`</div>`)
		// Search results render below the field as ordinary rows.
		for _, child := range item.Children {
			renderItem(b, child)
		}
	default:
		renderRegular(b, item)
	}
}

func renderRegular(b *strings.Builder, item menuet.SnapshotItem) {
	hasSubtitle := len(item.Subtitle) > 0
	hasChildren := len(item.Children) > 0
	if hasSubtitle {
		b.WriteString(`<div class="menu-row" style="margin: 0 5px; padding: 5px 9px 6px 7px; border-radius: 6px; display: flex; align-items: flex-start; gap: 7px;">`)
	} else {
		b.WriteString(`<div class="menu-row" style="margin: 0 5px; padding: 0 9px 0 7px; min-height: 24px; border-radius: 6px; display: flex; align-items: center; gap: 7px; font-size: 13.5px; color: var(--text);">`)
	}

	// Checkmark gutter, always 14px wide so labels line up whether or not
	// the row has State set.
	if item.State {
		b.WriteString(`<span style="width: 14px; flex: none; color: var(--accent); font-weight: 700; font-size: 12px;">✓</span>`)
	} else {
		b.WriteString(`<span style="width: 14px; flex: none;"></span>`)
	}

	if hasSubtitle {
		b.WriteString(`<div style="flex: 1; min-width: 0;">`)
		b.WriteString(`<div style="display: flex; align-items: center; gap: 6px; font-size: 13.5px; color: var(--text);">`)
		renderLabel(b, item)
		b.WriteString(`</div>`)
		b.WriteString(`<div style="margin-top: 2px; font-size: 11px; color: var(--text-2);">`)
		for _, run := range item.Subtitle {
			renderRun(b, run, false)
		}
		b.WriteString(`</div>`)
		b.WriteString(`</div>`)
	} else {
		b.WriteString(`<span style="flex: 1;">`)
		renderLabel(b, item)
		b.WriteString(`</span>`)
		if item.Shortcut != nil {
			b.WriteString(`<span style="color: var(--text-2); letter-spacing: 0.04em;">`)
			b.WriteString(stdhtml.EscapeString(shortcutString(*item.Shortcut)))
			b.WriteString(`</span>`)
		} else if hasChildren {
			b.WriteString(`<svg width="7" height="11" viewBox="0 0 7 11" fill="none" style="opacity: 0.5;"><polyline points="1,1 6,5.5 1,10" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"></polyline></svg>`)
		}
	}
	b.WriteString(`</div>`)
}

// renderLabel emits either the Runs (per-segment styled) or the plain
// Text. Plain text inherits the row's font color; Runs bring their own.
func renderLabel(b *strings.Builder, item menuet.SnapshotItem) {
	if len(item.Runs) > 0 {
		for _, run := range item.Runs {
			renderRun(b, run, false)
		}
		return
	}
	b.WriteString(stdhtml.EscapeString(item.Text))
}
