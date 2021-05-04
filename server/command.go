package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/juanfran/mattermost-gather-users/server/utils"
)

// ExecuteCommand run command
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	adminCommands := []string{"add", "remove", "meetings", "set_meetings"}

	caller, err := p.API.GetUser(args.UserId)
	if err != nil {
		return nil, err
	}

	if len(split) <= 1 {
		// Invalid invocation, needs at least one sub-command
		return &model.CommandResponse{}, nil
	}

	msg := "This command is not supported"

	if split[1] == "on" {
		p.addUser(args.UserId)

		_, ok := p.usersMeetings[args.UserId]

		if !ok {
			p.usersMeetings[args.UserId] = []string{}
		}

		msg = "Gather plugin activate, wait for a meeting."
	} else if split[1] == "off" {
		p.removeUser(args.UserId)

		msg = "Gather plugin deactivate."
	} else if split[1] == "pause" {
		p.paused = append(p.paused, args.UserId)
		p.persistPausedUsers()
		msg = "Gather plugin paused."
	} else if split[1] == "info" {
		config := p.getConfiguration()

		if !caller.IsSystemAdmin() && !config.AllowInfoForEveryone  {
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         "Only system admins can do this.",
			}, nil
		}

		var lines []string
		for _, userId := range p.users {
			user, err := p.API.GetUser(userId)
			if err != nil {
				return nil, err
			}
			lines = append(lines, fmt.Sprintf(" - %s %s (@%s)\n", user.FirstName, user.LastName, user.Username))
		}

		sort.Strings(lines)

		var msgBuilder strings.Builder
		msgBuilder.WriteString("Users signed up for coffee meetings:\n")
		for _, line := range lines {
			msgBuilder.WriteString(line)
		}
		msg = msgBuilder.String()
	} else if utils.Contains(adminCommands, split[1]) {
		if !caller.IsSystemAdmin() {
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         "Only system admins can do this.",
			}, nil
		}

		if split[1] == "add" {
			for _, v := range args.UserMentions {
				p.addUser(v)
			}

			msg = "Add complete."
		} else if split[1] == "remove" {
			for _, v := range args.UserMentions {
				user, _ := p.API.GetUser(v)
				p.removeUser(user.Id)
			}

			msg = "Remove complete."
		} else if split[1] == "meetings" {
			mettings := p.usersMeetingsByUsername()
			output, _ := json.Marshal(mettings)

			msg = "```" + string(output) + "```"
		} else if split[1] == "set_meetings" {
			byt := []byte(split[2])
			dat := make(map[string][]string)
			mettings := make(map[string][]string)

			if err := json.Unmarshal(byt, &dat); err != nil {
				msg += "\nFailed parsing json."
			} else {
				for _, userId := range p.users {

					_, ok := mettings[userId]
					if !ok {
						mettings[userId] = []string{}
					}

					userData, _ := p.API.GetUser(userId)

					if dat[userData.Username] != nil {
						for _, userName := range dat[userData.Username] {
							user, _ := p.API.GetUserByUsername(userName)
							mettings[userId] = append(mettings[userId], user.Id)
						}
					}
				}

				p.usersMeetings = mettings
				p.persistMeetings()

				msg = "Meetings setted"
			}
		}
	} else if split[1] == "on" || split[1] == "off" {
		// Save users when list changed
		err := p.persistUsers()
		if err != nil {
			msg += "\nFailed to save list of users, contact your administrator."
		}
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         msg,
	}, nil
}
