package event

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// EventMsg wraps an Event for the Bubble Tea message system.
type EventMsg struct {
	Event Event
}

// ListenForEvents returns a tea.Cmd (subscription) that reads one Event from
// the FIFO channel and returns it as an EventMsg. Wire it into the model's
// Init or Update to keep receiving events in a loop:
//
//	func (a App) Init() tea.Cmd {
//	    return event.ListenForEvents(ctx, ch)
//	}
//
//	case event.EventMsg:
//	    // handle msg.Event
//	    return a, event.ListenForEvents(ctx, ch)
func ListenForEvents(ctx context.Context, ch <-chan Event) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev, ok := <-ch:
			if !ok {
				// Channel closed — signal TUI that the FIFO is gone.
				return FIFOClosedMsg{}
			}
			return EventMsg{Event: ev}
		case <-ctx.Done():
			return FIFOClosedMsg{}
		}
	}
}

// FIFOClosedMsg is sent when the FIFO channel closes (TUI should stop subscribing).
type FIFOClosedMsg struct{}
