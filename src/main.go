package main

import (
	"encoding/json"
	"os"
)

var Config struct {
	Port  int  `json:"port"`
	Debug bool `json:"debug"`
}

func main() {
	// Read configuration
	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(content, &Config); err != nil {
		panic(err)
	}

	ServerListen()
}
