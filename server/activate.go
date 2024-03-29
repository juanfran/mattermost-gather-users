package main

import (
	"encoding/json"

	"github.com/mattermost/mattermost-server/v5/model"
)

// OnActivate activate pluguin
func (p *Plugin) OnActivate() error {
	p.addCronFunc()

	bot := &model.Bot{
		Username:    "gather-users",
		DisplayName: "gatherUsers",
	}
	botUserID, ensureBotErr := p.Helpers.EnsureBot(bot)

	if ensureBotErr != nil {
		return ensureBotErr
	}

	p.botUserID = botUserID

	// Deserialize user data
	userData, err := p.API.KVGet("users")
	if err != nil {
		return err
	}
	if userData != nil {
		var users []string
		err := json.Unmarshal(userData, &users)
		if err != nil {
			p.users = []string{}
		} else {
			p.users = users
		}
	}

	// Deserialize paused data
	pausedData, err := p.API.KVGet("paused")
	if err != nil {
		return err
	}
	if pausedData != nil {
		var paused []string
		err := json.Unmarshal(pausedData, &paused)
		if err != nil {
			p.paused = []string{}
		} else {
			p.paused = paused
		}
	}

	// Deserialize meetings data
	meetingsData, err := p.API.KVGet("meetings")
	if err != nil {
		return err
	}

	p.usersMeetings = make(map[string][]string)

	if meetingsData != nil {
		meetings := make(map[string][]string)
		err := json.Unmarshal(meetingsData, &meetings)
		if err != nil {
			p.usersMeetings = make(map[string][]string)
		} else {
			p.usersMeetings = meetings
		}
	}

	// Deserialize oddUserTurn data
	oddUserTurnData, err := p.API.KVGet("oddUserTurn")
	if err != nil {
		return err
	}

	p.oddUserTurn = []string{}

	if oddUserTurnData != nil {
		oddUserTurn := []string{}
		err := json.Unmarshal(oddUserTurnData, &oddUserTurn)
		if err != nil {
			p.oddUserTurn = []string{}
		} else {
			p.oddUserTurn = oddUserTurn
		}
	}

	return p.API.RegisterCommand(&model.Command{
		Trigger:          "gather-plugin",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: on, off, pause",
	})
}
