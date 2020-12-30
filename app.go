package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/densestvoid/groupme"
	"github.com/gorilla/mux"
)

// App simple application interface to allow starting and stopping
type App interface {
	Start() (finishedSignal chan struct{}, err error)
	Stop()
}

// app contains all of the pieces for the bots with a convient start and stop function
type app struct {
	gmClient   *groupme.Client
	discSesh   *discordgo.Session
	server     http.Server
	finishSig  chan struct{}
	config     *Config
	userLookup map[string]string
}

// Rerturn a new App instance
func NewApp(config *Config) App {
	return &app{
		config: config,
	}
}

// Start will start both bots and run until stop is called or the server fails
func (a *app) Start() (finshedSignal chan struct{}, err error) {
	a.userLookup = map[string]string{}
	a.finishSig = make(chan struct{})
	a.gmClient = groupme.NewClient("")
	a.discSesh, err = discordgo.New("Bot " + a.config.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session %W", err)
	}

	if err := a.discSesh.Open(); err != nil {
		a.discSesh = nil
		return nil, fmt.Errorf("failed to open discord session %W", err)
	}
	a.AddDiscordHandlers()
	a.AddGroupMeHandlers()

	a.server.Addr = "0.0.0.0:8000"
	go func() {
		if err := a.server.ListenAndServe(); err != nil {
			fmt.Println(err)
			a.Stop()
			// Stop the appliciation
		}
	}()

	const startMessage = "Top of the morning to you 🎩"
	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, startMessage, nil); err != nil {
		fmt.Println(err)
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.DiscordChannelID, startMessage); err != nil {
		fmt.Println(err)
	}
	return a.finishSig, nil
}

// Stop close connections and stop the application
func (a *app) Stop() {
	const stopMessage = " Bye Bye 👋"
	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, stopMessage, nil); err != nil {
		fmt.Println(err)
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.DiscordChannelID, stopMessage); err != nil {
		fmt.Println(err)
	}

	if a.discSesh != nil {
		a.discSesh.Close()
	}
	if err := a.server.Close(); err != nil {
		fmt.Println(err)
	}
	if a.finishSig != nil {
		close(a.finishSig)
	}

}

// AddDiscordHandlers add all handlers for the discord session
func (a *app) AddDiscordHandlers() {
	a.discSesh.AddHandler(func(session *discordgo.Session, msg *discordgo.MessageCreate) {
		if msg.Author.Bot {
			return
		}
		if resp, ok := a.parseDiscordCommand(msg); ok {
			if _, err := a.discSesh.ChannelMessageSend(a.config.DiscordChannelID, resp); err != nil {
				fmt.Println(err)
			}
			return
		}

		userName := a.getUsername(msg.Message)
		textMessage := fmt.Sprintf("[%s]: %s", userName, msg.Content)

		if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, textMessage, nil); err != nil {
			fmt.Println(err)
		}
	})
	a.discSesh.AddHandler(func(session *discordgo.Session, msg *discordgo.MessageUpdate) {
		if msg.Author.Bot {
			return
		}

		userName := a.getUsername(msg.Message)
		textMessage := fmt.Sprintf("[%s]*EDIT*: %s", userName, msg.Content)

		if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, textMessage, nil); err != nil {
			fmt.Println(err)
		}
	})
}

// AddGroupMeHandlers add all GroupMe handlers for the client
func (a *app) AddGroupMeHandlers() {
	router := mux.NewRouter()
	a.server.Handler = router
	router.Methods("POST").Path("/GroupMeEvents").HandlerFunc(func(respWriter http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		bytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			fmt.Println(err)
		}

		var msg groupme.Message
		if err := json.Unmarshal(bytes, &msg); err != nil {
			fmt.Println(err)
		}
		if msg.SenderType != groupme.SenderType_User {
			return
		}

		textMessage := fmt.Sprintf("[%s]: %s", msg.Name, msg.Text)

		if _, err := a.discSesh.ChannelMessageSend(a.config.DiscordChannelID, textMessage); err != nil {
			fmt.Println(err)
		}
	})
}

func (a *app) parseDiscordCommand(msg *discordgo.MessageCreate) (string, bool) {
	if !strings.HasPrefix(msg.Content, "!") {
		return "", false
	}
	cmdList := strings.Split(msg.Content[1:], " ")
	switch cmdList[0] {
	case "update":
		return a.discordUpdateCommand(cmdList, msg), true
	case "":
		return "Try one of these if you do not know what to do!\n    update: let's you update your info", true
	}

	return fmt.Sprintf("😫  D'oh! '%s' is not a valid command", cmdList[0]), true
}

func (a *app) discordUpdateCommand(cmdList []string, msg *discordgo.MessageCreate) string {
	if len(cmdList) < 2 {
		return "Avalible options: \n    name: Change the name that shows up in GroupMe\n"
	}
	switch cmdList[1] {
	case "name":
		newName := strings.Join(cmdList[2:], " ")
		oldName := a.getUsername(msg.Message)
		err := a.updateUserName(msg.Message, newName)
		if err != nil {
			return err.Error()
		}
		return fmt.Sprintf("'%s' is now '%s'", oldName, newName)
	default:
		return fmt.Sprintf("😫  D'oh! '%s' is not a valid command", cmdList[1])
	}
}

func (a *app) updateUserName(msg *discordgo.Message, newName string) error {
	a.userLookup[msg.Author.Username] = newName
	return nil
}

func (a *app) getUsername(msg *discordgo.Message) string {
	userName, ok := a.userLookup[msg.Author.Username]
	if ok {
		return userName
	}
	if msg.Member.Nick != "" {
		return msg.Member.Nick
	}
	return msg.Author.Username
}
