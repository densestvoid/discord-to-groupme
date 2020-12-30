package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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
	gmClient  *groupme.Client
	discSesh  *discordgo.Session
	server    http.Server
	finishSig chan struct{}
	config    *Config
}

// Rerturn a new App instance
func NewApp(config *Config) App {
	return &app{
		config: config,
	}
}

// Start will start both bots and run until stop is called or the server fails
func (a *app) Start() (finshedSignal chan struct{}, err error) {
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

	const startMessage = "<--- Started listening --->"
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
	const stopMessage = "<--- Stopped listening --->"
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

		textMessage := fmt.Sprintf("[%s]: %s", msg.Author.Username, msg.Content)

		if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, textMessage, nil); err != nil {
			fmt.Println(err)
		}
	})
	// a.discSesh.AddHandler(func(session *discordgo.Session, msg *discordgo.MessageCreate) {
	// 	if msg.Author.Bot {
	// 		return
	// 	}

	// 	textMessage := fmt.Sprintf("[%s]: %s", msg.Author.Username, msg.Content)

	// 	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, textMessage, nil); err != nil {
	// 		fmt.Println(err)
	// 	}
	// })
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
