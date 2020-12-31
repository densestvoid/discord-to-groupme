package discord_to_groupme

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/densestvoid/groupme"
)

// Config - defines the tokens used to connect to GroupMe and Discord,
// as well as the messages the bot sends
type Config struct {
	Filename string `json:"-"`

	GroupMeBotToken groupme.ID
	Discord         DiscordConfig

	StartupMessage  string
	ShutdownMessage string
}

type DiscordConfig struct {
	BotToken                 string
	SyncChannelID            string
	AdminChannelID           string
	TroubleshootingChannelID string
}

// ReadConfig - opens the file with filename and tries the parse the json in a Config
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
	if err := json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}
	config.Filename = filename

	if !config.GroupMeBotToken.Valid() {
		return nil, fmt.Errorf("invalid GroupMe Bot Token: %s", config.GroupMeBotToken)
	}

	return &config, nil
}
