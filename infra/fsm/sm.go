package fsm

import "github.com/samber/mo"

type State struct {
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

	// Try getting next state. Return None if event is invalid.
	//
	// Read-only
	TryGetNextState(event Event) mo.Result[State]

	// Transfer to next state. Return None if event is invalid.
	//
	// Modify state.
	Transfer(event Event) mo.Result[State]
}
