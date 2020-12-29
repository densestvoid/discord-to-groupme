package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/densestvoid/groupme"
)

type Config struct {
	GroupMeBotToken  groupme.ID
	DiscordBotToken  string
	DiscordChannelID string
}

func ReadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	return &config, json.Unmarshal(bytes, &config)
}
