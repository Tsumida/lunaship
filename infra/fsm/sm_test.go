package fsm

import (
	"fmt"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
)

type event struct {
	id     string
	source string
	target string
	etype  string
	body   string
}

// impl Event interface
func (e event) ID() string {
	return e.id
}

func (e event) EventSource() string {
	return e.source
}

func (e event) EventReceiver() string {
	return e.target
}

func (e event) EventType() string {
	return e.etype
}

func (e event) EventDesc() string {
	return "mock"
}

func (e event) Body() []byte {
	return []byte(e.body)
}

func (e event) ToByte() []byte {
	return nil
}

func (e event) FromByte([]byte) mo.Result[Resource] {
	return mo.Err[Resource](fmt.Errorf("not implemented"))
}

func (e event) ResourceType() string {
	return "event"
}

var _ (Event) = event{}

type sm struct {
	current State

	initState State
	endStates []State
	m         map[string]State
}

func hashStateAndEvent(state State, event Event) string {
	return fmt.Sprintf("%s#%s#%s", state.StateID, event.EventType(), event.ID())
}

func (s *sm) InitState() State {
	return State{}
}

func (s *sm) EndStates() []State {
	return []State{}
}

func (s *sm) State() State {
	return s.current
}

func (s *sm) TryGetNextState(event Event) mo.Result[State] {
	key := hashStateAndEvent(s.current, event)
	v, ok := s.m[key]
	if !ok {
		return mo.Err[State](StateMachineErr{
			Trigger: event,
			ErrType: ErrTypeInvalidEvent,
		})
	}

	return mo.Ok[State](v)
}

func (s *sm) SetState(state State) mo.Result[State] {
	s.current = state
	return mo.Ok[State](state)
}

func (s *sm) Transfer(event Event) mo.Result[State] {
	res := s.TryGetNextState(event)
	if res.IsError() {
		return res
	}

	s.current = res.MustGet()

	return mo.Ok[State](s.current)
}

func (s *sm) ID() string {
	return "0"
}

func (s *sm) ResourceType() string {
	return "sm"
}

func (s *sm) ToByte() []byte {
	return []byte(s.ID())
}

func (s *sm) FromByte([]byte) mo.Result[Resource] {
	return mo.Ok[Resource](&sm{})
}

var _ (StateMachine) = (*sm)(nil)

func TestXxx(t *testing.T) {

	var (
		smID   = "abcdef"
		source = "1"
		target = smID

		s1 = State{StateID: "inited"}
		s2 = State{StateID: "ended"}
		s3 = State{StateID: "processing"}
		s4 = State{StateID: "done"}
		s5 = State{StateID: "failed"}

		e1 = event{
			id:     "1",
			source: source,
			target: target,
			etype:  "event_process",
			body:   "",
		}
		e2 = event{
			id:     "2",
			source: source,
			target: target,
			etype:  "event_done",
			body:   "",
		}
		e3 = event{
			id:     "3",
			source: source,
			target: target,
			etype:  "event_failed",
			body:   "",
		}

		eEnd = event{
			id:     "4",
			source: source,
			target: target,
			etype:  "event_end",
			body:   "",
		}
		e4 = event{
			id:     "5",
			source: source,
			target: target,
			etype:  "event_retry",
			body:   "",
		}
	)

	sm1 := sm{
		initState: State{StateID: "inited"},
		endStates: []State{{StateID: "ended"}},
		current:   State{StateID: "inited"},
		m: map[string]State{
			hashStateAndEvent(s1, e1):   s3,
			hashStateAndEvent(s3, e2):   s4,
			hashStateAndEvent(s3, e3):   s5,
			hashStateAndEvent(s4, eEnd): s2,
			hashStateAndEvent(s5, eEnd): s2,
			hashStateAndEvent(s5, e4):   s1,
		},
	}

	assert.Equal(t, State{StateID: "inited"}, sm1.State())

	r1 := sm1.Transfer(e1)
	assert.True(t, r1.IsOk())
	assert.Equal(t, State{StateID: "processing"}, r1.MustGet())

	r2 := sm1.Transfer(e1)
	assert.True(t, r2.IsError())

	r3 := sm1.Transfer(e2)
	assert.True(t, r3.IsOk())
	assert.Equal(t, State{StateID: "done"}, r3.MustGet())
}
