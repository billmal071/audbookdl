package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
)

// HelpOverlay shows context-sensitive help.
type HelpOverlay struct {
	visible  bool
	bindings []key.Binding
}

// Toggle flips the overlay visibility and sets the displayed bindings.
func (h *HelpOverlay) Toggle(bindings []key.Binding) {
	h.visible = !h.visible
	h.bindings = bindings
}

// View renders the help overlay or returns an empty string when not visible.
func (h *HelpOverlay) View() string {
	if !h.visible {
		return ""
	}
	var view string
	view += titleStyle.Render("Keyboard Shortcuts") + "\n\n"
	for _, b := range h.bindings {
		view += fmt.Sprintf("  %s  %s\n",
			sourceStyle.Render(b.Help().Key),
			subtitleStyle.Render(b.Help().Desc))
	}
	view += "\n" + subtitleStyle.Render("Press ? to close")
	return view
}
