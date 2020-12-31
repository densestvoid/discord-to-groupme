package discord_to_groupme

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/densestvoid/groupme"
)

// Message - used to parse commands whether it is a GroupMe or Discord message
type Message interface {
	GetText() string
	GetUsername() string
}

// DiscordMessage - wrapper used to add Message interface functions
type DiscordMessage struct {
	*discordgo.Message
}

// GetText - satisfies the Message interface
func (m DiscordMessage) GetText() string { return m.Content }

// GetUsername - satisfies the Message interface
func (m DiscordMessage) GetUsername() string {
	if m.Member.Nick != "" {
		return m.Member.Nick
	}
	return m.Author.Username
}

// GroupMeMessage - wrapper used to add Message interface functions
type GroupMeMessage struct {
	*groupme.Message
}

// GetText - satisfies the Message interface
func (m GroupMeMessage) GetText() string { return m.Text }

// GetUsername - satisfies the Message interface
func (m GroupMeMessage) GetUsername() string { return m.Name }

const invalidCommand string = `😫  D'oh! "%s" is not a valid command`

const syncCommandHelp string = `Try one of these if you do not know what to do!
	update: let's you update your info
`

func (a *app) syncMessageParse(msg Message) (string, bool) {
	text := msg.GetText()
	if !strings.HasPrefix(text, "!") {
		return "", false
	}
	cmdList := strings.Split(text[1:], " ")
	switch cmdList[0] {
	case "update":
		return a.updateCommand(cmdList, msg), true
	case "":
		return syncCommandHelp, true
	}

	return fmt.Sprintf(invalidCommand, cmdList[0]), true
}

const adminCommandHelp string = `Try one of these if you do not know what to do!
	pause: stops syncing messages between Discord and GroupMe
	unpuase: resumes syncing messages between Discord and GroupMe
	reload: reloads the config file and reconnects the dicord client
`

func (a *app) adminMessageParse(msg Message) (string, bool) {
	text := msg.GetText()
	if !strings.HasPrefix(text, "!") {
		return "", false
	}
	cmdList := strings.Split(text[1:], " ")
	switch cmdList[0] {
	case "pause":
		return a.pauseCommand(), true
	case "unpause":
		return a.unpauseCommand(), true
	case "reload":
		return a.reloadConfig(), true
	case "":
		return adminCommandHelp, true
	}

	return fmt.Sprintf(invalidCommand, cmdList[0]), true
}

func (a *app) updateCommand(cmdList []string, msg Message) string {
	if len(cmdList) < 2 {
		return "Avalible options: \n    name: Change the name that shows up in GroupMe\n"
	}
	switch cmdList[1] {
	case "name":
		newName := strings.Join(cmdList[2:], " ")
		oldName := a.getUsername(msg)
		err := a.updateUserName(msg, newName)
		if err != nil {
			return err.Error()
		}
		return fmt.Sprintf("'%s' is now '%s'", oldName, newName)
	default:
		return fmt.Sprintf(invalidCommand, cmdList[1])
	}
}

func (a *app) pauseCommand() string {
	if a.paused {
		return "Syncing already paused"
	}

	a.paused = true
	text := "Syncing has been paused"

	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, text, nil); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, text); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	return text
}

func (a *app) unpauseCommand() string {
	if !a.paused {
		return "Syncing already not paused"
	}

	a.paused = false
	text := "Syncing has been unpaused"

	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, text, nil); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, text); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	return text
}

func (a *app) reloadConfig() string {
	newConfig, err := ReadConfig(a.config.Filename)
	if err != nil {
		return fmt.Sprintf("Failed to read config: %s", err)
	}

	discSesh, err := a.ConfigureDiscordSession(newConfig)
	if err != nil {
		return fmt.Sprintf("Failed to update config: %s", err)
	}

	if err := a.discSesh.Close(); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	a.config = newConfig
	a.discSesh = discSesh
	return "Updated config"
}

func (a *app) updateUserName(msg Message, newName string) error {
	a.userLookup[msg.GetUsername()] = newName
	return nil
}

func (a *app) getUsername(msg Message) string {
	userName, ok := a.userLookup[msg.GetUsername()]
	if ok {
		return userName
	}
	return msg.GetUsername()
}
