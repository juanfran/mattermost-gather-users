package main

import (
	"encoding/json"
	"fmt"
)

func (p *Plugin) persistUsers() error {
	// Persist currently signed-up users
	userData, err := json.Marshal(p.users)
	if err != nil {
		p.API.LogError(fmt.Sprintf("Failed to serialize users: %s", err.Error()))
		return err
	}

	// Cannot reuse `err` here, because `KVSet` returns a pointer, not an interface,
	// which when cast to the `error` interface of `err`, will result in a non-nil value.
	err2 := p.API.KVSet("users", userData)

	if err2 != nil {
		p.API.LogError(fmt.Sprintf("Failed to persist users: %s", err2.Error()))
		return err2
	}
	return nil
}

func (p *Plugin) persistPausedUsers() error {
	// Persist currently signed-up paused
	pausedData, err := json.Marshal(p.paused)
	if err != nil {
		p.API.LogError(fmt.Sprintf("Failed to serialize paused paused: %s", err.Error()))
		return err
	}

	// Cannot reuse `err` here, because `KVSet` returns a pointer, not an interface,
	// which when cast to the `error` interface of `err`, will result in a non-nil value.
	err2 := p.API.KVSet("paused", pausedData)

	if err2 != nil {
		p.API.LogError(fmt.Sprintf("Failed to persist paused: %s", err2.Error()))
		return err2
	}
	return nil
}

func (p *Plugin) persistMeetings() error {
	// Persist currently signed-up meetings
	usersMeetings, err := json.Marshal(p.usersMeetings)
	if err != nil {
		p.API.LogError(fmt.Sprintf("Failed to serialize users meetings: %s", err.Error()))
		return err
	}

	// Cannot reuse `err` here, because `KVSet` returns a pointer, not an interface,
	// which when cast to the `error` interface of `err`, will result in a non-nil value.
	err2 := p.API.KVSet("meetings", usersMeetings)

	if err2 != nil {
		p.API.LogError(fmt.Sprintf("Failed to persist users meetings: %s", err2.Error()))
		return err2
	}
	return nil
}
