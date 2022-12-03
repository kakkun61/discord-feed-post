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
	"github.com/mmcdole/gofeed"
	"gopkg.in/yaml.v3"
)

const appName = "discord-feed-post"

func main() {
	config, err := readConfig()
	var feed gofeed.Feed
	err = json.NewDecoder(os.Stdin).Decode(&feed)
	if err != nil {
		log.Fatal(fmt.Errorf("Failed to parse a feed. Caused by %w", err))
	}
	webhookUrl, ok := (*config)[feed.Link]
	if !ok {
		log.Fatal(fmt.Errorf("Not found a webhook URI: %s.", feed.Link))
	}
	requestBodyBytes, err := json.Marshal(convertFeedToDiscordRequest(feed))
	if !ok {
		log.Fatal(fmt.Errorf("Failed to marshal feed. Caused by %w", err))
	}
	resp, err := http.Post(webhookUrl, "application/json", bytes.NewReader(requestBodyBytes))
	if err != nil {
		log.Fatal(fmt.Errorf("Failed to request an HTTP post. Caused by %w", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		bodyBytes, err := io.ReadAll(resp.Request.Body)
		if err == nil {
			log.Fatal(fmt.Errorf(`A status code for an HTTP post is not 200: %s: "%s".`, resp.Status, string(bodyBytes)))
		} else {
			log.Fatal(fmt.Errorf("A status code for an HTTP post is not 200: %s.", resp.Status))
		}
	}
}

// Config is a map from a feed ID to webhook URI
type Config = map[string]string

func readConfig() (*Config, error) {
	configPath := filepath.Join(xdg.ConfigHome, appName, "config.yaml")
	configFile, err := openFileAndCreateIfNecessaryRecursive(configPath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("Failed to open a config: %s. Caused by %w", configPath, err)
	}
	defer configFile.Close()
	configBytes, err := io.ReadAll(configFile)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("Failed to read a config: %s. Caused by %w", configPath, err)
	}
	config, err := unmarshalConfig(configBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal a config: %s. Caused by %w", configPath, err)
	}
	return config, nil
}

func unmarshalConfig(bytes []byte) (*Config, error) {
	var config Config
	err := yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal the config. Caused by %w", err)
	}
	return &config, nil
}

func openFileAndCreateIfNecessaryRecursive(path string, flag int, mode os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(path, flag, mode)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("Failed to open the file: %s. Caused by %w", path, err)
		} else {
			file, err = os.Create(path)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("Failed to create the file: %s. Caused by %w", path, err)
				}
				dir := filepath.Dir(path)
				err = os.MkdirAll(dir, 0755)
				if err != nil {
					return nil, fmt.Errorf("Failed to create the director: %s. Caused by %w", dir, err)
				}
				file, err = os.Create(path)
				if err != nil {
					return nil, fmt.Errorf("Failed to create the file after creating the directory: %s. Caused by %w", path, err)
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
func convertFeedToDiscordRequest(feed gofeed.Feed) DiscordRequestBody {
	var body DiscordRequestBody
	body.Username = feed.Title
	body.Embeds = make([]DiscordRequestEmbed, minInt(len(feed.Items), 10))
	for i := 0; i < len(body.Embeds); i++ {
		embed := &(body.Embeds[i])
		item := feed.Items[len(body.Embeds)-1-i]
		embed.Title = item.Title
		embed.Description = item.Description
		embed.Url = item.Link
		embed.Timestamp = string(item.Published)
		embed.Author.Name = item.Author.Name
	}
	return body
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
