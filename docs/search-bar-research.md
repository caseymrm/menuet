# In-menu search bar — design notes & investigation log

The `menuet.Search` MenuItem type renders an `NSSearchField` inside an
`NSMenu` (à la Apple's Help search). Getting that to feel right —
visible text cursor, live filtering, arrow-key navigation of results,
Enter to activate, Esc to dismiss, remembered query across opens — was
substantially harder than it sounds because `NSMenu`'s tracking loop is
walled off from the standard `NSResponder` chain in a way Apple never
exposed publicly. This document captures what we learned so the next
person to touch this (probably future-Casey) doesn't have to re-derive
it from scratch.

## tl;dr — what actually shipped

- **Text cursor & focus** require flipping a private flag on the popup
  window via `-[NSPopupMenuWindow setKeyOverride:YES]` plus invoking
  `-[NSCell selectWithFrame:inView:editor:delegate:start:length:]` on
  the field's cell to engage a non-modal editing session.
- **Type-to-filter** flows naturally once the field is first responder
  and the input context is engaged — `controlTextDidChange:` fires on
  each keystroke and we call back into Go for fresh result items.
- **Enter activates the first result; Esc closes the menu** — both
  routed through `-[NSControlTextEditingDelegate control:textView:
  doCommandBySelector:]`.
- **Query persistence across opens**: saved in
  `viewDidMoveToWindow` whenever the SearchView's window goes nil
  (catches submenu close even when the root menu's tracking continues)
  and again in `NSMenuDidEndTrackingNotification` as a safety net. The
  saved value is restored by `populate:` when the menu reopens, with
  the field's full range selected so the next keystroke replaces it.
- **Mid-tracking elision** of long result strings is avoided by
  forcing `NSLineBreakByClipping` in each item's attributed title
  (bypasses the elide) and growing the menu's `minimumWidth` to
  whichever item's `NSMenuItemCell.cellSize.width` is widest. The
  width "ratchets up" within a session and never shrinks (jarring).
- **Arrow-key navigation** of result items is NOT supported. We
  determined this requires synthesizing a hardware-level mouse click
  (`CGEventPost`) on the field to promote it to NSMenu's actively-
  selected item state. The trade-offs (Accessibility permission
  prompt, visible mouse cursor warp, fragility under user mouse
  movement during the menu) were worse than just not supporting
  arrow nav. The full investigation is preserved in the "What didn't
  work" section below in case a cleaner approach surfaces later.

## Mac App Store implications

`setKeyOverride:` is private API. Apps that use `menuet.Search` will be
rejected by Mac App Store review. The Go-side doc comment on `Search`
calls this out explicitly. Direct-distribution apps (the menuet
ecosystem so far — whyawake, notafan, traytter, etc.) are unaffected.
We did **not** ship the click-sim arrow-key approach, so no
`CGEventPost` and no Accessibility permission prompt on first run.

## What didn't work

Each of these was tried during the investigation. Listed so we don't
re-tread the same ground.

### Suppressing the visible cursor jump

| Attempt | Result |
| --- | --- |
| `[NSCursor hide]` before warp | Apple's docs say `hide` is reinstated on next mouse movement; `CGWarpMouseCursorPosition` counts as movement, so the hide gets undone immediately. Confirmed. |
| `CGDisplayHideCursor(kCGDirectMainDisplay)` | Bypassed by AppKit's own cursor management during menu tracking — cursor stays visible regardless. |
| `CGDisplayHideCursor` on every display | Same as above. |
| Restoring the cursor mid-tracking (after click registered) | NSMenu sees the cursor leave the field and revokes the "field actively selected" state — arrow nav breaks. |
| `CGAssociateMouseAndMouseCursorPosition(false)` to freeze the visible cursor while warping the logical one | Started down this path but `setHiddenUntilMouseMoves:` after warp made it unnecessary; not fully tested. |
| Synthetic `mouseDown` via `[NSApp postEvent:]` | NSMenu's tracking loop reinterprets the queued event as a "click outside menu" and dismisses. |
| Synthetic `mouseDown` via `[NSApp sendEvent:]` | The synchronous dispatch enters AppKit's modal mouse-tracking loop and **blocks for 5+ seconds** waiting for a natural mouse-up that never arrives. |
| Skip the warp entirely; post `CGEventPost` with the event's `.location` set to the field | Works only when the cursor happens to already be inside the field rect (sometimes true on first open, never reliable). Confirmed via diagnostic distance logging. |
| Trigger the click on first keystroke (`controlTextDidChange:`) instead of on menu open | Click is absorbed silently; NSMenu's "field active" state is only settable during initial tracking-setup, not mid-session. |
| Trigger the click in `control:textShouldBeginEditing:` | Same — by the time editing begins, tracking is settled; click does not promote the field. |
| `accessibilityPerformPress` on the field | NSSearchField advertises zero AX actions including `AXPress`. Not on the table. |

### Other failed angles

| Attempt | Result |
| --- | --- |
| Override `_allowsActiveInputContextDuringMenuTracking` via `method_setImplementation` swizzle | Swizzle installs cleanly but AppKit never queries the method. Despite the suggestive name, it's not the gate. |
| Swizzle `_handleKeyEvent:` on `NSPopupMenuWindow` to capture arrow keys | Swizzle fires for **letter** keys (a, c, Tab, etc.) but **not** for arrow keys (keyCodes 125/126). Arrows are intercepted by NSMenu's tracking event loop at a layer below Obj-C method dispatch — almost certainly plain C functions inside libAppKit that aren't on any class's method table. |
| `NSEvent.addLocalMonitorForEventsMatchingMask:` for keyDown events | Doesn't fire during NSMenu tracking. Even with `isKey=YES` forced via `setKeyOverride:`. |
| `[window makeKeyWindow]` to coax the popup window into a state where standard event routing works | No-op. NSPopupMenuWindow overrides `makeKeyWindow` to deny the request during tracking. |
| `accessibilityPerformPress` on the field | Already mentioned — field advertises no AX actions. |

### `objc_runtime` probe

We dumped every method on `NSMenu`, `NSMenuItem`, `NSPopupMenuWindow`,
`NSCarbonMenuImpl`, and related classes matching `search` / `filter` /
`highlight` / `track` / `key` / `first` / `focus`. The probe code is at
`docs/research/nsmenu_probe.m` (kept for future investigations). It is
how `setKeyOverride:` and `_handleKeyEvent:` were discovered.

## Why arrow handling is fundamentally inaccessible

`_handleKeyEvent:` is the AppKit private method that handles letter
keys during menu tracking — we swizzled it and proved it fires for
letters. But the same swizzle never sees keyCodes 125/126 (Down/Up).
That tells us arrow keys are processed **before** Obj-C method dispatch
gets involved — they're handled by static C functions inside libAppKit
that NSMenu's tracking loop calls directly from its `CGEvent`-pulling
inner loop. Those functions aren't registered with the Obj-C runtime
and don't show up in any class dump.

Apple's own Help search works because Apple is **inside** AppKit — they
can call the private NSMenu-internal C functions (like the
`_selectNextItem:` family) directly. We can't from outside the
framework.

The synthetic-click workaround sidesteps this by making NSMenu's
tracking loop *think the user selected the field item*, after which
NSMenu's own arrow-key handling routes them at the field's editor
instead of at result-item navigation. We're hijacking the right state
machine, just from a different entry point.

## Why the click has to be on menu open, not on first keystroke

The intuition is "trigger the cursor jump only when the user actually
types, so people who don't search never see the jump." This *does not
work* — confirmed empirically. The click event is silently absorbed
when fired any time after tracking has settled. The "field is the
active menu item" state appears to be set once during NSMenu's tracking
setup and can only be promoted/demoted via cursor-driven hover events
or programmatic input *within* the framework. So we have to do the
click on `viewDidMoveToWindow`'s non-nil call, with the trade-off that
the cursor briefly jumps even for users who never touch the search.

## Future avenues (untried)

1. **Disassemble libAppKit** with Hopper / lldb to find the actual C
   function that handles arrow keys during menu tracking. If we find
   it and can locate the entry point via runtime symbol lookup, calling
   it directly would replace the click-sim hack. High effort, brittle
   across OS versions.
2. **Reverse-engineer Apple's Help menu** specifically. Their search
   field clearly uses private API we couldn't surface via runtime
   introspection. Inspecting the actual implementation via lldb on a
   running TextEdit or similar might reveal the entry point.
3. **`CGAssociateMouseAndMouseCursorPosition(false)` + `CGWarp`**.
   Decouples visible cursor from logical cursor — the warp would move
   the logical position (which NSMenu uses for hit-testing) without
   the visible cursor moving. We started this path but the
   `setHiddenUntilMouseMoves:` solution arrived first and made it
   unnecessary. Worth revisiting if the current behavior ever breaks.

## Files involved

- `menuet.m` — `MenuetSearchView` class (the bulk of the implementation)
- `menuet.go` — `searchResults` cgo export and `Application.searchResults`
- `menuitem.go` — `Search` concrete type implementing `MenuItem`
- `cmd/catalog/catalog.go` — demo of `menuet.Search` over US states
- `docs/research/nsmenu_probe.m` — runtime introspection tool used to
  discover private selectors

## macOS version sensitivity

All behaviors documented above were observed on macOS Sequoia (15.x —
build label "Darwin 24.6.0"). NSMenu's tracking loop has been broadly
stable since the early Carbon-to-Cocoa transition, but the specific
private selectors (`setKeyOverride:`, `_handleKeyEvent:`) and the
mechanics around `setHiddenUntilMouseMoves:` could change. If the
feature breaks in a future macOS, this document and the probe code at
`docs/research/nsmenu_probe.m` are the starting point.
