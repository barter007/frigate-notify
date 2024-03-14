package config

import (
	"log"
	"os"

	"github.com/kkyr/fig"
)

type Config struct {
	Frigate Frigate `fig:"frigate"`
	Alerts  Alerts  `fig:"alerts"`
	Monitor Monitor `fig:"monitor"`
}

type Frigate struct {
	Server   string  `fig:"server" validate:"required"`
	Insecure bool    `fig:"ignoressl" default:false`
	WebAPI   WebAPI  `fig:"webapi"`
	MQTT     MQTT    `fig:"mqtt"`
	Cameras  Cameras `fig:"cameras"`
}

type WebAPI struct {
	Enabled  bool `fig:"enabled" default:false`
	Interval int  `fig:"interval" default:30`
}

type MQTT struct {
	Enabled  bool   `fig:"enabled" default:false`
	Server   string `fig:"server" default:""`
	Port     int    `fig:"port" default:1883`
	ClientID string `fig:"clientid" default:"frigate-notify"`
	Username string `fig:"username" default:""`
	Password string `fig:"password" default:""`
}

type Cameras struct {
	Exclude []string `fig:"exclude" default:[]`
}

type Alerts struct {
	General General `fig:"general"`
	Discord Discord `fig:"discord"`
	Gotify  Gotify  `fig:"gotify"`
	SMTP    SMTP    `fig:"smtp"`
}

type General struct {
	Title string `fig:"title" default:"Frigate Alert"`
}

type Discord struct {
	Enabled bool   `fig:"enabled" default:false`
	Webhook string `fig:"webhook" default:""`
}

type Gotify struct {
	Enabled  bool   `fig:"enabled" default:false`
	Server   string `fig:"server" default:""`
	Token    string `fig:"token" default:""`
	Insecure bool   `fig:"ignoressl" default:false`
}

type SMTP struct {
	Enabled   bool   `fig:"enabled" default:false`
	Server    string `fig:"server" default:""`
	Port      int    `fig:"port" default:25`
	TLS       bool   `fig:"tls" default:false`
	User      string `fig:"user" default:""`
	Password  string `fig:"password" default:""`
	Recipient string `fig:"recipient" default:""`
}

type Monitor struct {
	Enabled  bool   `fig:"enabled" default:false`
	URL      string `fig:"url" default:""`
	Interval int    `fig:"interval" default:60`
	Insecure bool   `fig:"ignoressl" default:false`
}

var ConfigData Config

// loadConfig opens & attempts to parse configuration file
func LoadConfig(configFile string) {
	// Set config file location
	if configFile == "" {
		var ok bool
		configFile, ok = os.LookupEnv("FN_CONFIGFILE")
		if !ok {
			configFile = "./config.yml"
		}
	}

	// Load Config file
	log.Print("Loading config file: ", configFile)
	err := fig.Load(&ConfigData, fig.File(configFile), fig.UseEnv("FN"))
	if err != nil {
		log.Fatal("Failed to load config file! Error: ", err)
	}

	// Send config file to validation before completing
	validateConfig()

	log.Print("Config file loaded.")
}

// validateConfig checks config file structure & loads info into associated packages
func validateConfig() {
	var configErrors []string
	log.Println("Validating config file...")

	if (ConfigData.Frigate.WebAPI.Enabled && ConfigData.Frigate.MQTT.Enabled) || (!ConfigData.Frigate.WebAPI.Enabled && !ConfigData.Frigate.MQTT.Enabled) {
		configErrors = append(configErrors, "Please configure only one polling method: Frigate Web API or MQTT")
	}

	// Check for camera exclusions
	if len(ConfigData.Frigate.Cameras.Exclude) > 0 {
		log.Println("Cameras to exclude from alerting:")
		for _, c := range ConfigData.Frigate.Cameras.Exclude {
			log.Println(" -", c)
		}
	}

	// Check MQTT Config
	if ConfigData.Frigate.MQTT.Enabled {
		log.Println("MQTT Enabled.")
		if ConfigData.Frigate.MQTT.Server == "" {
			configErrors = append(configErrors, "No MQTT server address specified!")
		}
		if ConfigData.Frigate.MQTT.Username != "" && ConfigData.Frigate.MQTT.Password == "" {
			configErrors = append(configErrors, "MQTT user provided, but no password!")
		}
		if ConfigData.Frigate.MQTT.Port == 0 {
			ConfigData.Frigate.MQTT.Port = 1883
		}
	}

	// Check / Load alerting configuration
	if ConfigData.Alerts.Discord.Enabled {
		log.Print("Discord alerting enabled.")
		if ConfigData.Alerts.Discord.Webhook == "" {
			configErrors = append(configErrors, "No Discord webhook specified!")
		}
	}
	if ConfigData.Alerts.Gotify.Enabled {
		log.Print("Gotify alerting enabled.")
		if ConfigData.Alerts.Gotify.Server == "" {
			configErrors = append(configErrors, "No Gotify server specified!")
		}
		if ConfigData.Alerts.Gotify.Token == "" {
			configErrors = append(configErrors, "No Gotify token specified!")
		}
	}
	if ConfigData.Alerts.SMTP.Enabled {
		log.Print("SMTP alerting enabled.")
		if ConfigData.Alerts.SMTP.Server == "" {
			configErrors = append(configErrors, "No SMTP server specified!")
		}
		if ConfigData.Alerts.SMTP.Recipient == "" {
			configErrors = append(configErrors, "No SMTP recipients specified!")
		}
		if ConfigData.Alerts.SMTP.User != "" && ConfigData.Alerts.SMTP.Password == "" {
			configErrors = append(configErrors, "SMTP username in config, but no password provided!")
		}
		if ConfigData.Alerts.SMTP.Port == 0 {
			ConfigData.Alerts.SMTP.Port = 25
		}
	}

	// Validate monitoring config
	if ConfigData.Monitor.Enabled {
		log.Println("App monitoring enabled.")
		if ConfigData.Monitor.URL == "" {
			configErrors = append(configErrors, "App monitor enabled but no URL specified!")
		}
		if ConfigData.Monitor.Interval == 0 {
			ConfigData.Monitor.Interval = 60
		}
	}

	if len(configErrors) > 0 {
		log.Println("Config validation failed:")
		for _, msg := range configErrors {
			log.Println(" -", msg)
		}
		log.Fatal("Please fix config errors before restarting app.")
	} else {
		log.Println("Config file validated!")
	}
}
