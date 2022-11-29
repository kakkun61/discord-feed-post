package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/wbernest/atom-parser"
	"golang.org/x/tools/blog/atom"
	"gopkg.in/yaml.v3"
)

const appName = "discord-feed-post"

func main() {
	config, err := readConfig()
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(Error{"Failed to read the stdin.", err})
	}
	feed, err := atomparser.ParseString(string(stdinBytes))
	if err != nil {
		log.Fatal(Error{"Failed to parse a feed.", err})
	}
	webhookUrl, ok := (*config)[feed.ID]
	if !ok {
		log.Fatal(Error{"Not found a webhook URI: " + feed.ID + ".", nil})
	}
	requestBodyBytes, err := json.Marshal(convertFeedToDiscordRequest(*feed))
	if !ok {
		log.Fatal(Error{"Failed to marshal feed.", err})
	}
	resp, err := http.Post(webhookUrl, "application/json", bytes.NewReader(requestBodyBytes))
	if err != nil {
		log.Fatal(Error{"Failed to request an HTTP post.", err})
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		bodyBytes, err := io.ReadAll(resp.Request.Body)
		if err == nil {
			log.Fatal(Error{fmt.Sprintf(`A status code for an HTTP post is not 200: %s: "%s".`, resp.Status, string(bodyBytes)), nil})
		} else {
			log.Fatal(Error{fmt.Sprintf("A status code for an HTTP post is not 200: %s.", resp.Status), nil})
		}
	}
}

// Config is a map from a feed ID to webhook URI
type Config = map[string]string

func readConfig() (*Config, error) {
	configPath := filepath.Join(xdg.ConfigHome, appName, "config.yaml")
	configFile, err := openFileAndCreateIfNecessaryRecursive(configPath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, Error{"Failed to open a config: " + configPath, err}
	}
	defer configFile.Close()
	configBytes, err := io.ReadAll(configFile)
	if err != nil && err != io.EOF {
		return nil, Error{"Failed to read a config: " + configPath, err}
	}
	config, err := unmarshalConfig(configBytes)
	if err != nil {
		return nil, Error{"Failed to unmarshal a config: " + configPath, err}
	}
	return config, nil
}

func unmarshalConfig(bytes []byte) (*Config, error) {
	var config Config
	err := yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, Error{"Failed to unmarshal the config.", err}
	}
	return &config, nil
}

func openFileAndCreateIfNecessaryRecursive(path string, flag int, mode os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(path, flag, mode)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, Error{path + ": Failed to open the file.", err}
		} else {
			file, err = os.Create(path)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, Error{path + ": Failed to create the file.", err}
				}
				dir := filepath.Dir(path)
				err = os.MkdirAll(dir, 0755)
				if err != nil {
					return nil, Error{dir + ": Failed to create the directory.", err}
				}
				file, err = os.Create(path)
				if err != nil {
					return nil, Error{path + ": Failed to create the file after creating the directory.", err}
				}
			}
		}
	}
	return file, nil
}

type DiscordRequestBody struct {
	Username string                `json:"username"`
	Embeds   []DiscordRequestEmbed `json:"embeds"`
}

type DiscordRequestEmbed struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Url         string        `json:"url"`
	Timestamp   string        `json:"timestamp"` // ISO 8601
	Author      DiscordAuthor `json:"author"`
}

type DiscordAuthor struct {
	Name string `json:"name"`
}

// a max number of entries is 10.
func convertFeedToDiscordRequest(feed atom.Feed) DiscordRequestBody {
	var body DiscordRequestBody
	body.Username = feed.Title
	body.Embeds = make([]DiscordRequestEmbed, minInt(len(feed.Entry), 10))
	for i := 0; i < len(body.Embeds); i++ {
		embed := &(body.Embeds[i])
		entry := feed.Entry[len(body.Embeds)-1-i]
		embed.Title = entry.Title
		embed.Description = entry.Summary.Body
		for j := 0; j < len(entry.Link); j++ {
			if entry.Link[j].Rel == "" {
				embed.Url = entry.Link[j].Href
			}
		}
		embed.Timestamp = string(entry.Published)
		embed.Author.Name = entry.Author.Name
	}
	return body
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Error struct {
	Message string
	Origin  error
}

func (e Error) Error() string {
	if e.Origin == nil {
		return e.Message
	}
	return e.Message + " Caused by: " + e.Origin.Error()
}
