package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
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

	cron        *cron.Cron
	cronEntryID cron.EntryID

	users         []string
	paused         []string
	usersMeetings map[string][]string
	meetInCron    []string

	botUserID string
}

// Meeting the way to store meeting
type Meeting struct {
	User1 string `json:"user1"`
	User2 string `json:"user2"`
}

// OnActivate activate pluguin
func (p *Plugin) OnActivate() error {
	c := cron.New()
	p.cron = c
	p.cron.Start()

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
			p.users = users;
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
			p.paused = paused;
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
			p.usersMeetings = meetings;
		}
	}

	return p.API.RegisterCommand(&model.Command{
		Trigger:          "gather-plugin",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: on, off, pause",
	})
}

// UserHasLeftTeam one user left the team
func (p *Plugin) UserHasLeftTeam(c *plugin.Context, teamMember *model.TeamMember) {
	p.users = remove(p.users, teamMember.UserId)
	p.removeUserMeetings(teamMember.UserId)
	p.persistMeetings()
}

func (p *Plugin) refreshCron(configuration *configuration) {
	p.cron.Remove(p.cronEntryID)
	p.addCronFunc()
}

func (p *Plugin) addCronFunc() {
	config := p.getConfiguration()
	configCron := config.Cron

	if configCron == "" {
		configCron = "@weekly"
	}

	if config.Cron == "custom" && len(config.CustomCron) > 0 {
		configCron = config.CustomCron
	}

	// every minute "* * * * *"
	p.cronEntryID, _ = p.cron.AddFunc(configCron, func() {
		p.runMeetings()
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

func (p *Plugin) hasRemeaningMeetings(userId string) bool {
	availableUsers := p.getAvailableUsers()
	availableUsersSize := len(availableUsers)

	return len(p.usersMeetings[userId]) < (availableUsersSize - 1)
}

func (p *Plugin) printMeetInCron() {
	result := []string{}

	for _, userId := range p.meetInCron {
		userData, _ := p.API.GetUser(userId)
		result = append(result, userData.Username)
	}

	fmt.Println("printMeetInCron", result)
}

func (p *Plugin) runMeetings() {
	p.cleanUsers()

	p.meetInCron = []string{}

	availableUsers := p.getAvailableUsers()
	usersWithoutPendingMeetings := []string{}

	shuffleUsers(availableUsers)

	for _, userId := range availableUsers {
		_, ok := p.usersMeetings[userId]
		if !ok {
			p.usersMeetings[userId] = []string{}
		}

		if p.hasRemeaningMeetings(userId) {
			userToMeet, ok := p.findUserToMeet(userId)

			if ok {
				p.startMeeting(userId, userToMeet)
			}
		} else {
			usersWithoutPendingMeetings = append(usersWithoutPendingMeetings, userId)
		}
	}

	for _, userId := range usersWithoutPendingMeetings {
		userToMeet, ok := p.findUserToMeet(userId)

		if ok {
			p.startMeeting(userId, userToMeet)
		}
	}

	for _, userId := range usersWithoutPendingMeetings {
		userToMeet, ok := p.findAnyUserToMeet(userId)

		if ok {
			p.startMeeting(userId, userToMeet)
		}
	}

	p.persistMeetings()
}

func removeUserMeeting(meetings []string, userID string) []string {
	var result []string

	for _, user := range meetings {
		if user != userID {
			result = append(result, user)
		}
	}

	return result
}

func (p *Plugin) userHasMeetings(userID string) bool {
	return len(p.usersMeetings[userID]) > 0
}

func (p *Plugin) removeUserMeetings(userID string) {
	for _, i := range p.users {
		p.usersMeetings[i] = removeUserMeeting(p.usersMeetings[i], userID)
	}

	delete(p.usersMeetings, userID);
}

func (p *Plugin) getAvailableUsers() []string {
	var users []string

	for _, userId := range p.users {
		if !contains(p.paused, userId) {
			users = append(users, userId)
		}
	}

	return users
}

func (p *Plugin) isUserInTheCurrentCron(userID string) bool {
	return contains(p.meetInCron, userID)
}

func (p *Plugin) findUserToMeet(userID string) (string, bool) {
	if p.isUserInTheCurrentCron(userID) {
		return "", false
	}

	availableUsers := p.getAvailableUsers()
	userMeetings := p.usersMeetings[userID]

	shuffleUsers(availableUsers)

	// find if the user haven't meet someone
	for _, pairUserID := range availableUsers {
		if pairUserID != userID &&
			!p.isUserInTheCurrentCron(pairUserID) &&
			!contains(userMeetings, pairUserID) {
			return pairUserID, true
		}
	}

	if !p.hasRemeaningMeetings(userID) &&
	   len(userMeetings) > 0 &&
	   !p.isUserInTheCurrentCron(userMeetings[0]) {
		return userMeetings[0], true
	}

	return "", false
}

func (p *Plugin) findAnyUserToMeet(userID string) (string, bool) {
	if p.isUserInTheCurrentCron(userID) {
		return "", false
	}

	availableUsers := p.getAvailableUsers()
	shuffleUsers(availableUsers)

	for _, pairUserID := range availableUsers {
		if pairUserID != userID && !p.isUserInTheCurrentCron(pairUserID) {
			return pairUserID, true
		}
	}

	return "", false
}

func  (p *Plugin) cleanUsers() {
	availableUsers := p.getAvailableUsers()

	mettings := make(map[string][]string)

	for _, user := range p.users {
		_, ok := mettings[user]
		if !ok {
			mettings[user] = []string{}
		}

		for _, userId := range p.usersMeetings[user] {
			if contains(availableUsers, userId) {
				mettings[user] = append(mettings[user], userId)
			}
		}
	}

	p.usersMeetings = mettings
}

func  (p *Plugin) usersMeetingsByUsername() map[string][]string {
	mettings := make(map[string][]string)

	for _, user := range p.users {
		mainUserData, _ := p.API.GetUser(user)
		_, ok := mettings[mainUserData.Username]
		if !ok {
			mettings[mainUserData.Username] = []string{}
		}

		for _, userMeeting := range p.usersMeetings[user] {
			userData, _ := p.API.GetUser(userMeeting)
			mettings[mainUserData.Username] = append(mettings[mainUserData.Username], userData.Username)
		}
	}

	return mettings
}

func (p *Plugin) startMeeting(userID string, pairUserID string) {
	newUserMeetings := removeUserMeeting(p.usersMeetings[userID], pairUserID)
	p.usersMeetings[userID] = append(newUserMeetings, pairUserID)

	newUserMeetings = removeUserMeeting(p.usersMeetings[pairUserID], userID)
	p.usersMeetings[pairUserID] = append(newUserMeetings, userID)

	p.meetInCron = append(p.meetInCron, userID, pairUserID)

	users := []string{p.botUserID, userID, pairUserID}

	channel, _ := p.API.GetGroupChannel(users)

	config := p.getConfiguration()

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channel.Id,
		Message:   config.InitText,
	}

	p.persistMeetings()
	p.API.CreatePost(post)
}

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

func (p *Plugin) addUser(userID string) {
	if !contains(p.users, userID) {
		p.users = append(p.users, userID)

		config := p.getConfiguration()

		// meet now only if the user has no previous meetings
		if config.FirstMeeting && !p.userHasMeetings(userID) {
			userToMeet, ok := p.findUserToMeet(userID)

			if ok {
				p.startMeeting(userID, userToMeet)
			}
		}
	}
}

func (p *Plugin) removeUser(userID string) {
	p.users = remove(p.users, userID)
	p.meetInCron = remove(p.meetInCron, userID)
	p.removeUserMeetings(userID)
	p.persistMeetings()
}

// OnDeactivate deactivate plugin
func (p *Plugin) OnDeactivate() error {
	p.cron.Remove(p.cronEntryID)
	p.cron.Stop()

	return p.persistUsers()
}

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
	} else if contains(adminCommands, split[1]) {
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
