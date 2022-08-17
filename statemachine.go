package main

import (
	sm "github.com/lni/dragonboat/v4/statemachine"
)

type ByStateMachine struct {
	store *store
}

func NewByStateMachine(uint64, uint64) sm.IOnDiskStateMachine {
	return &ByStateMachine{
		store: NewStore("example-store"), //<- @TODO not this
	}
}

func (s ByStateMachine) Lookup(q any) (any, error) {
	cmd := q.(Command)
	return s.store.Get(cmd.Part, cmd.Id)
}

func (s ByStateMachine) Open() {
	
}

func (s ByStateMachine) Close() error {
	return nil
}
