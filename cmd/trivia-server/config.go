package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"unicode"
)

var configPathFlag = flag.String("config", "", "The location of the config file. If this argument is not provided the paths './trivia-config.json' and './config/trivia-config.json' are searched in that order.")

type triviaConfig struct {
	DB struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
		SSLMode  string `json:"sslMode"`
	} `json:"db"`

	Auth struct {
		Pepper256 string `json:"pepper256"`
	} `json:"auth"`

	Server struct {
		Addr            string `json:"addr"`
		ShutdownTimeout string `json:"shutdownTimeout"`
	} `json:"server"`
}

func loadConfig() *triviaConfig {
	var configPath string
	if configPathFlag != nil {
		configPath = strings.TrimSpace(*configPathFlag)
	}

	foundPath := false
	var usePath string
	if len(configPath) > 0 {
		if _, err := os.Stat(configPath); err != nil {
			log.Fatal("error opening config file: ", err)
		}
		usePath = configPath
		foundPath = true
	}

	if !foundPath {
		if _, err := os.Stat("./trivia-config.json"); err == nil {
			usePath = "./trivia-config.json"
			foundPath = true
		}
	}

	if !foundPath {
		if _, err := os.Stat("./config/trivia-config.json"); err == nil {
			usePath = "./config/trivia-config.json"
			foundPath = true
		}
	}

	if !foundPath {
		log.Fatal("No config file found.")
	}

	fmt.Println("Config File: ", foundPath)
	configBytes, err := ioutil.ReadFile(usePath)
	if err != nil {
		log.Fatal("error reading config file: ", err)
	}

	config := triviaConfig{}
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("error parsing config file json: ", err)
	}

	return &config
}

func getStringValue(s string) (string, bool) {
	if len(s) > 0 {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) > 0 {
			return trimmed, true
		}
	}
	return "", false
}

// requireString make sure that a string is not empty. If an empty string is provided
// and there is a default value, the default value is returned. If there is an empty string
// and not default value, the program panics with the given error string.
func requireStringValue(s string, def string, errString string) string {
	if len(s) > 0 {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) > 0 {
			return trimmed
		}
	}

	if len(def) > 0 {
		return def
	}

	log.Fatal(errString)
	return "" // picnic
}

func escapeDBValue(unescaped string) string {
	escaped := unescaped
	quoteString := false

	for _, r := range escaped {
		if unicode.IsSpace(r) {
			quoteString = true
			break
		}
	}

	if quoteString {
		escaped = strings.Replace(escaped, "\\", "\\\\", -1)
		escaped = strings.Replace(escaped, "'", "\\'", -1)
		return "'" + escaped + "'"
	}
	return escaped
}

func createSQLConnectionString(config *triviaConfig) string {
	var settings = make([]string, 0)
	settings = append(settings, fmt.Sprintf("user=%s", escapeDBValue(requireStringValue(config.DB.User, "", "db.user cannot be empty"))))
	settings = append(settings, fmt.Sprintf("dbname=%s", escapeDBValue(requireStringValue(config.DB.Name, "", "db.name cannot be empty"))))
	settings = append(settings, fmt.Sprintf("sslmode=%s", escapeDBValue(requireStringValue(config.DB.SSLMode, "disable", "db.sslmode should have a default"))))

	password, ok := getStringValue(config.DB.Password)
	if ok {
		settings = append(settings, fmt.Sprintf("password=%s", escapeDBValue(password)))
	}

	host, ok := getStringValue(config.DB.Host)
	if ok {
		settings = append(settings, fmt.Sprintf("host=%s", escapeDBValue(host)))
	}

	port, ok := getStringValue(config.DB.Port)
	if ok {
		settings = append(settings, fmt.Sprintf("port=%s", escapeDBValue(port)))
	}

	return strings.Join(settings, " ")
}
