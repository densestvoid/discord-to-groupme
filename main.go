package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/densestvoid/groupme"
	"github.com/gorilla/mux"
)

func main() {
	var configFilename string
	flag.StringVar(&configFilename, "c", "tokens.json", "specifies the name of the config file to use")
	flag.Parse()
	config, err := ReadConfig(configFilename)
	if err != nil {
		fmt.Println(err)
		return
	}

	gmClient := groupme.NewClient("")

	dClient, err := discordgo.New("Bot " + config.DiscordBotToken)
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := dClient.Open(); err != nil {
		fmt.Println(err)
	}
	defer dClient.Close()

	dClient.AddHandler(func(session *discordgo.Session, msg *discordgo.MessageCreate) {
		if msg.Author.Bot {
			return
		}

		textMessage := fmt.Sprintf("[%s]: %s", msg.Author.Username, msg.Content)

		if err := gmClient.PostBotMessage(config.GroupMeBotToken, textMessage, nil); err != nil {
			fmt.Println(err)
		}
	})

	router := mux.NewRouter()
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

		if _, err := dClient.ChannelMessageSend(config.DiscordChannelID, textMessage); err != nil {
			fmt.Println(err)
		}
	})

	if err := gmClient.PostBotMessage(config.GroupMeBotToken, "<--- Started listening --->", nil); err != nil {
		fmt.Println(err)
	}

	if _, err := dClient.ChannelMessageSend(config.DiscordChannelID, "<--- Started listening --->"); err != nil {
		fmt.Println(err)
	}

	server := http.Server{
		Addr:    "0.0.0.0:8000",
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	if err := server.Close(); err != nil {
		fmt.Println(err)
	}

	if err := gmClient.PostBotMessage(config.GroupMeBotToken, "<--- Stopped listening --->", nil); err != nil {
		fmt.Println(err)
	}

	if _, err := dClient.ChannelMessageSend(config.DiscordChannelID, "<--- Stopped listening --->"); err != nil {
		fmt.Println(err)
	}
}
