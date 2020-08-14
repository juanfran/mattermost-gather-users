package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/robfig/cron/v3"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	cron          *cron.Cron
	cronEntryID   cron.EntryID
	users         []string
	usersMeetings []Meeting
	meetInCron    []string
}

// Meeting the way to store meeting
type Meeting struct {
	user1 string
	user2 string
}

// OnActivate activate pluguin
func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()

	c := cron.New()
	p.cron = c
	p.cron.Start()
	configCron := config.cron

	if configCron == "" {
		configCron = "@weekly"
	}

	p.cronEntryID, _ = c.AddFunc("* * * * *", func() {
		runMeetings(p)
	})

	return p.API.RegisterCommand(&model.Command{
		Trigger:          "gather-plugin",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: on, off",
	})
}

func shuffleUsers(a []string) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
}

func contains(slice []string, e string) bool {
	for _, a := range slice {
		if a == e {
			return true
		}
	}
	return false
}

func remove(slice []string, toRemove string) []string {
	for i, v := range slice {
		if v == toRemove {
			slice = append(slice[:i], slice[i+1:]...)
			break
		}
	}

	return slice
}

func runMeetings(p *Plugin) {
	fmt.Println("|||| Meetings starting")

	p.meetInCron = []string{}
	shuffleUsers(p.users)

	// user with remaining meetings
	for _, userID := range p.users {
		if !contains(p.meetInCron, userID) {
			userToMeet, ok := findUserToMeet(p, userID)

			if ok {
				startMeeting(p, userID, userToMeet)
			}
		}
	}

	// empty user meetings if it is already full & try to meet
	for _, userID := range p.users {
		if !contains(p.meetInCron, userID) {
			_, ok := findUserToMeet(p, userID)

			if !ok {
				p.usersMeetings = removeUserMeetings(p.usersMeetings, userID)
			}

			userToMeet, ok := findUserToMeet(p, userID)

			if ok {
				startMeeting(p, userID, userToMeet)
			}
		}
	}
}

func removeUserMeetings(meetings []Meeting, userID string) []Meeting {
	result := []Meeting{}

	for i := range meetings {
		if meetings[i].user1 != userID && meetings[i].user2 != userID {
			result = append(result, meetings[i])
		}
	}

	return result
}

func userHasMeetings(meetings []Meeting, userID string) bool {
	for _, meeting := range meetings {
		if meeting.user1 == userID || meeting.user2 == userID {
			return true
		}
	}

	return false
}

func findUserToMeet(p *Plugin, userID string) (string, bool) {
	for _, pairUserID := range p.users {
		if pairUserID != userID &&
			!contains(p.meetInCron, pairUserID) &&
			!meetingExist(p.usersMeetings, userID, pairUserID) {
			return pairUserID, true
		}
	}

	return "", false
}

func meetingExist(meetings []Meeting, userID string, pairUserID string) bool {
	for _, meeting := range meetings {
		if (meeting.user1 == userID && meeting.user2 == pairUserID) ||
			(meeting.user2 == userID && meeting.user1 == pairUserID) {
			return true
		}
	}

	return false
}

func startMeeting(p *Plugin, userID string, pairUserID string) {
	p.usersMeetings = append(p.usersMeetings, Meeting{userID, pairUserID})
	p.meetInCron = append(p.meetInCron, userID, pairUserID)
	fmt.Println("|||| Starting meeting between " + userID + " and " + pairUserID)
}

// OnDeactivate desativate plugin
func (p *Plugin) OnDeactivate() error {
	p.cron.Remove(p.cronEntryID)
	p.cron.Stop()
	return nil
}

// ExecuteCommand run command
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	if len(split) > 1 && (split[1] == "on" || split[1] == "off") {
		msg := "Gather plugin activate, wait for a meeting."

		if split[1] == "off" {
			msg = "Gather plugin deactivate."

			p.users = remove(p.users, args.UserId)
		} else if !contains(p.users, args.UserId) {
			p.users = append(p.users, args.UserId)

			// meet now only if the user has no previous meetings
			if !userHasMeetings(p.usersMeetings, args.UserId) {
				userToMeet, ok := findUserToMeet(p, args.UserId)

				if ok {
					startMeeting(p, args.UserId, userToMeet)
				}
			}
		}

		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         msg,
		}, nil
	}

	return &model.CommandResponse{}, nil
}
