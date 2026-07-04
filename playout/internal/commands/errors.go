package commands

import "fmt"

// RejectedError is returned by a command handler to signal a business-logic
// rejection (e.g. skip on a mandatory item). The Dispatcher converts this into
// a CommandRejected event with Accepted=false instead of a handler error.
type RejectedError struct {
	Reason string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("command rejected: %s", e.Reason)
}
