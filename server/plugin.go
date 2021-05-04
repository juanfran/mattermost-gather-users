package main

import (
	"fmt"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/robfig/cron/v3"
	"github.com/juanfran/mattermost-gather-users/server/utils"
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
	oddUserTurn []string

	meetInCron    []string
	oddUserInCron string

	botUserID string
}

// Meeting the way to store meeting
type Meeting struct {
	User1 string `json:"user1"`
	User2 string `json:"user2"`
}

// UserHasLeftTeam one user left the team
func (p *Plugin) UserHasLeftTeam(c *plugin.Context, teamMember *model.TeamMember) {
	p.users = utils.Remove(p.users, teamMember.UserId)
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

func (p *Plugin) fillOddUserTurnList() {
	for _, userId := range p.users {
		if !utils.Contains(p.oddUserTurn, userId) {
			p.oddUserTurn = append(p.oddUserTurn, userId)
		}
	}
}

func (p *Plugin) runMeetings() {
	p.cleanUsers()

	p.meetInCron = []string{}
	p.oddUserInCron = ""
	usersWithoutPendingMeetings := []string{}

	availableUsers := p.getAvailableUsers()
	isOdd := (len(availableUsers) % 2) != 0

	if isOdd {
		p.fillOddUserTurnList()
		p.oddUserInCron = p.oddUserTurn[0]
		p.oddUserTurn = utils.Remove(p.oddUserTurn, p.oddUserInCron)
		p.oddUserTurn = append(p.oddUserTurn, p.oddUserInCron)
		p.persistOddUserTurn()
	}

	availableUsers = p.getAvailableUsers()

	utils.ShuffleUsers(availableUsers)

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


func (p *Plugin) userHasMeetings(userID string) bool {
	return len(p.usersMeetings[userID]) > 0
}

func (p *Plugin) removeUserMeetings(userID string) {
	for _, i := range p.users {
		p.usersMeetings[i] = utils.Remove(p.usersMeetings[i], userID)
	}

	delete(p.usersMeetings, userID);
}

func (p *Plugin) getAvailableUsers() []string {
	var users []string

	for _, userId := range p.users {
		if !utils.Contains(p.paused, userId) && userId != p.oddUserInCron {
			users = append(users, userId)
		}
	}

	return users
}

func (p *Plugin) isUserInTheCurrentCron(userID string) bool {
	return utils.Contains(p.meetInCron, userID)
}

func (p *Plugin) findUserToMeet(userID string) (string, bool) {
	if p.isUserInTheCurrentCron(userID) {
		return "", false
	}

	availableUsers := p.getAvailableUsers()
	userMeetings := p.usersMeetings[userID]

	utils.ShuffleUsers(availableUsers)

	// find if the user haven't meet someone
	for _, pairUserID := range availableUsers {
		if pairUserID != userID &&
			!p.isUserInTheCurrentCron(pairUserID) &&
			!utils.Contains(userMeetings, pairUserID) {
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
	utils.ShuffleUsers(availableUsers)

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
			if utils.Contains(availableUsers, userId) {
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
	newUserMeetings := utils.Remove(p.usersMeetings[userID], pairUserID)
	p.usersMeetings[userID] = append(newUserMeetings, pairUserID)

	newUserMeetings = utils.Remove(p.usersMeetings[pairUserID], userID)
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

func (p *Plugin) addUser(userID string) {
	if !utils.Contains(p.users, userID) {
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
	p.users = utils.Remove(p.users, userID)
	p.meetInCron = utils.Remove(p.meetInCron, userID)
	p.removeUserMeetings(userID)
	p.persistMeetings()
}

// OnDeactivate deactivate plugin
func (p *Plugin) OnDeactivate() error {
	p.cron.Remove(p.cronEntryID)
	p.cron.Stop()

	return p.persistUsers()
}
