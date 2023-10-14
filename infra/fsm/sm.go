package fsm

import (
	"fmt"

	"github.com/samber/mo"
)

type State struct {
	StateID string
}

var (
	ErrTypeInvalidEvent = "invalid event"
)

type StateMachineErr struct {
	Trigger Event
	ErrType string
}

func (e StateMachineErr) Error() string {
	return fmt.Sprintf("%s:%s", e.ErrType, string(e.Trigger.ToByte()))
}

type StateMachine interface {
	Resource

	// Return initial state.
	//
	// Read-only.
	InitState() State

	// Return end state sets.
	//
	// Read-only
	EndStates() []State

	// Return current state
	//
	// Read-only
	State() State

	// Try getting next state. Return None if event is invalid, else an StateMachineErr returned.
	//
	// Read-only
	TryGetNextState(event Event) mo.Result[State]

	// Transfer to next state. Return None if event is invalid, else an StateMachineErr returned.
	//
	// Modify state.
	Transfer(event Event) mo.Result[State]

	// Ignore transferring rules and set specified state. Return StateMachineErr if is invalid operation.
	//
	// Modify state.
	SetState(state State) mo.Result[State]
}
