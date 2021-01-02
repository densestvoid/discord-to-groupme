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
	update:	let's you update your info
	link:	manage cross platform account connections
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
	case "link":
		return a.linkCommand(cmdList[1:], msg), true
	case "":
		return syncCommandHelp, true
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

type Link struct {
	DiscordUsername string
	GroupMeUsername string
	Name            string
}

const linkCommandHelp string = `Available options:
	init:	connect your discord and groupme accounts.
			Creates one shared name, and allows mentioning cross platform
	accept:	accept the pending account link
	remove: cancel a pending or activate account link
`

func (a *app) linkCommand(cmdList []string, msg Message) string {
	if len(cmdList) == 0 {
		return linkCommandHelp
	}
	switch cmdList[0] {
	case "init":
		return a.linkInitCommand(cmdList[1:], msg)
	case "accept":
		return a.linkAcceptCommand(msg)
	case "remove":
		return a.linkRemoveCommand(msg)
	default:
		return fmt.Sprintf(invalidCommand, cmdList[0])
	}
}

func (a *app) linkInitCommand(cmdList []string, msg Message) string {
	if len(cmdList) < 2 {
		return "Must specify the account name to link to and the new name\n"
	}

	text := fmt.Sprintf("Link %s to %s with name %s?", msg.GetUsername(), cmdList[0], cmdList[1])

	switch msg.(type) {
	case DiscordMessage:
		if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, text, nil); err != nil {
			a.SendToTroubleshooting(err.Error())
			return "Failed to send link request"
		}
		a.pendingLinks = append(a.pendingLinks, &Link{msg.GetUsername(), cmdList[0], cmdList[1]})
	case GroupMeMessage:
		if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, text); err != nil {
			a.SendToTroubleshooting(err.Error())
			return "Failed to send link request"
		}
		a.pendingLinks = append(a.pendingLinks, &Link{cmdList[0], msg.GetUsername(), cmdList[1]})
	}

	return "Link request pending"
}

func (a *app) linkAcceptCommand(msg Message) string {
	text := "Linked account %s to %s"
	var pendingLink *Link
	for i, link := range a.pendingLinks {
		switch msg.(type) {
		case DiscordMessage:
			if link.DiscordUsername != msg.GetUsername() {
				continue
			}

			text = fmt.Sprintf(text, link.GroupMeUsername, link.DiscordUsername)
		case GroupMeMessage:
			if link.GroupMeUsername != msg.GetUsername() {
				continue
			}

			text = fmt.Sprintf(text, link.DiscordUsername, link.GroupMeUsername)
		}

		pendingLink = link
		a.accountLinks = append(a.accountLinks, link)
		a.pendingLinks = append(a.pendingLinks[:i], a.pendingLinks[i+1:]...)
		break
	}

	if pendingLink == nil {
		return fmt.Sprintf("There is no link request matching username %s", msg.GetUsername())
	}

	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, text, nil); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, text); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	return ""
}

func (a *app) linkRemoveCommand(msg Message) string {
	text := "Removed link between %s and %s"
	var linkToRemove *Link

	for i, link := range a.accountLinks {
		switch msg.(type) {
		case DiscordMessage:
			if link.DiscordUsername != msg.GetUsername() {
				continue
			}

			text = fmt.Sprintf(text, link.GroupMeUsername, link.DiscordUsername)
		case GroupMeMessage:
			if link.GroupMeUsername != msg.GetUsername() {
				continue
			}

			text = fmt.Sprintf(text, link.DiscordUsername, link.GroupMeUsername)
		}

		linkToRemove = link
		a.accountLinks = append(a.accountLinks[:i], a.accountLinks[i+1:]...)
		break
	}

	if linkToRemove == nil {
		for i, link := range a.pendingLinks {
			switch msg.(type) {
			case DiscordMessage:
				if link.DiscordUsername != msg.GetUsername() {
					continue
				}

				text = fmt.Sprintf(text, link.GroupMeUsername, link.DiscordUsername)
			case GroupMeMessage:
				if link.GroupMeUsername != msg.GetUsername() {
					continue
				}

				text = fmt.Sprintf(text, link.DiscordUsername, link.GroupMeUsername)
			}

			linkToRemove = link
			a.pendingLinks = append(a.pendingLinks[:i], a.pendingLinks[i+1:]...)
			break
		}

		if linkToRemove == nil {
			return fmt.Sprintf("There is no pending or active link matching username %s", msg.GetUsername())
		}
	}

	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, text, nil); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, text); err != nil {
		a.SendToTroubleshooting(err.Error())
	}

	return ""
}

const adminCommandHelp string = `Try one of these if you do not know what to do!
	pause: stops syncing messages between Discord and GroupMe
	unpuase: resumes syncing messages between Discord and GroupMe
	reload: reloads the config file and reconnects the dicord client
`

func (a *app) adminMessageParse(msg Message) (string, bool) {
	text := msg.GetText()
	fmt.Println(text)
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
	if link, exists := a.getLinkEntry(msg); exists {
		link.Name = newName
		return nil
	}

	switch msg.(type) {
	case DiscordMessage:
		a.discordUserLookup[msg.GetUsername()] = newName
	case GroupMeMessage:
		a.groupmeUserLookup[msg.GetUsername()] = newName
	}
	return nil
}

func (a *app) getUsername(msg Message) string {
	if link, exists := a.getLinkEntry(msg); exists {
		return link.Name
	}

	switch msg.(type) {
	case DiscordMessage:
		userName, ok := a.discordUserLookup[msg.GetUsername()]
		if ok {
			return userName
		}
	case GroupMeMessage:
		userName, ok := a.groupmeUserLookup[msg.GetUsername()]
		if ok {
			return userName
		}
	}
	return msg.GetUsername()
}

func (a *app) getLinkEntry(msg Message) (*Link, bool) {
	for _, link := range a.accountLinks {
		switch msg.(type) {
		case DiscordMessage:
			if msg.GetUsername() == link.DiscordUsername {
				return link, true
			}
		case GroupMeMessage:
			if msg.GetUsername() == link.GroupMeUsername {
				return link, true
			}
		}
	}
	return nil, false
}
