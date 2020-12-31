package discord_to_groupme

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
	gmClient   *groupme.Client
	discSesh   *discordgo.Session
	server     http.Server
	finishSig  chan struct{}
	config     *Config
	userLookup map[string]string
	paused     bool
}

// NewApp - Return a new App instance
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

	discSesh, err := a.ConfigureDiscordSession(a.config)
	if err != nil {
		return nil, err
	}
	a.discSesh = discSesh

	a.AddGroupMeHandlers()

	a.server.Addr = "0.0.0.0:8000"
	go func() {
		if err := a.server.ListenAndServe(); err != nil {
			fmt.Printf("Failed during Server ListenAndServe: %s\n", err)
			a.Stop()
			// Stop the appliciation
		}
	}()

	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, a.config.StartupMessage, nil); err != nil {
		fmt.Printf("Failed during PostBotMessage: %s\n", err)
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, a.config.StartupMessage); err != nil {
		fmt.Printf("Failed during Discord Session ChannelMessageSend: %s\n", err)
	}
	return a.finishSig, nil
}

// Stop close connections and stop the application
func (a *app) Stop() {
	if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, a.config.ShutdownMessage, nil); err != nil {
		fmt.Printf("Failed during GroupMe Client PostBotMessage: %s\n", err)
	}

	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, a.config.ShutdownMessage); err != nil {
		fmt.Printf("Failed during Discord Session ChannelMessageSend: %s\n", err)
	}

	if a.discSesh != nil {
		a.discSesh.Close()
	}
	if err := a.server.Close(); err != nil {
		fmt.Printf("Failed during Server Close: %s\n", err)
	}
	if a.finishSig != nil {
		close(a.finishSig)
	}
}

func (a *app) SendToTroubleshooting(s string) {
	if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.TroubleshootingChannelID, s); err != nil {
		fmt.Printf("Troubleshooting message: %s\nFailed to send:%s", s, err)
		a.Stop()
	}
}

func (a *app) ConfigureDiscordSession(config *Config) (*discordgo.Session, error) {
	discSesh, err := discordgo.New("Bot " + config.Discord.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session %W", err)
	}

	if err := discSesh.Open(); err != nil {
		return nil, fmt.Errorf("failed to open discord session %W", err)
	}

	a.AddDiscordHandlers(discSesh)
	return discSesh, nil
}

// AddDiscordHandlers add all handlers for the discord session
func (a *app) AddDiscordHandlers(discSesh *discordgo.Session) {
	discSesh.AddHandler(func(session *discordgo.Session, msg *discordgo.MessageCreate) {
		if msg.Author.Bot {
			return
		}

		discMsg := DiscordMessage{msg.Message}

		switch msg.ChannelID {
		case a.config.Discord.SyncChannelID:
			if resp, ok := a.syncMessageParse(discMsg); ok {
				if resp != "" {
					if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, resp); err != nil {
						a.SendToTroubleshooting(err.Error())
					}
				}
				return
			}
		case a.config.Discord.AdminChannelID:
			if resp, ok := a.adminMessageParse(discMsg); ok {
				if resp != "" {
					if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.AdminChannelID, resp); err != nil {
						a.SendToTroubleshooting(err.Error())
					}
				}
			}
			return
		default:
			return
		}

		if a.paused {
			return
		}

		userName := a.getUsername(discMsg)
		textMessage := fmt.Sprintf("[%s]: %s", userName, msg.Content)

		if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, textMessage, nil); err != nil {
			a.SendToTroubleshooting(err.Error())
		}
	})
	discSesh.AddHandler(func(session *discordgo.Session, msg *discordgo.MessageUpdate) {
		if a.paused || msg.Author.Bot || msg.ChannelID != a.config.Discord.SyncChannelID {
			return
		}

		discMsg := DiscordMessage{msg.Message}

		userName := a.getUsername(discMsg)
		textMessage := fmt.Sprintf("[%s]*EDIT*: %s", userName, msg.Content)

		if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, textMessage, nil); err != nil {
			a.SendToTroubleshooting(err.Error())
		}
	})
}

// AddGroupMeHandlers add all GroupMe handlers for the client
func (a *app) AddGroupMeHandlers() {
	router := mux.NewRouter()
	a.server.Handler = router
	router.Methods("POST").Path("/GroupMeEvents").HandlerFunc(func(respWriter http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		if a.paused {
			return
		}

		bytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			a.SendToTroubleshooting(err.Error())
		}

		var msg GroupMeMessage
		if err := json.Unmarshal(bytes, &msg); err != nil {
			a.SendToTroubleshooting(err.Error())
		}

		if msg.SenderType != groupme.SenderType_User {
			return
		}

		if resp, ok := a.syncMessageParse(msg); ok {
			if resp != "" {
				if err := a.gmClient.PostBotMessage(a.config.GroupMeBotToken, resp, nil); err != nil {
					a.SendToTroubleshooting(err.Error())
				}
			}
			return
		}

		if a.paused {
			return
		}

		userName := a.getUsername(msg)
		textMessage := fmt.Sprintf("[%s]: %s", userName, msg.GetText())

		if _, err := a.discSesh.ChannelMessageSend(a.config.Discord.SyncChannelID, textMessage); err != nil {
			a.SendToTroubleshooting(err.Error())
		}
	})
}
