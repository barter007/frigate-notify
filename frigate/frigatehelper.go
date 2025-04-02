package frigate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/0x2142/frigate-notify/config"
	"github.com/0x2142/frigate-notify/models"
	"github.com/0x2142/frigate-notify/util"
	"github.com/rs/zerolog/log"
)

func ValidateFrigateConnectivity() {
	var c = config.ConfigData
	var response []byte
	//var cookies []*http.Cookie
	var err error

	url := c.Frigate.Server
	max_attempts := c.Frigate.StartupCheck.Attempts
	interval := c.Frigate.StartupCheck.Interval

	// Test connectivity to Frigate
	log.Debug().Msg("Checking connection to Frigate server...")

	current_attempt := 1
	if max_attempts == 0 {
		max_attempts = 5
	}
	if interval == 0 {
		interval = 30
	}

	LoginToFrigateIfRequired(true)

	statsAPI := fmt.Sprintf("%s/api/stats", url)
	for current_attempt < max_attempts {
		response, err = util.HTTPGet(statsAPI, c.Frigate.Insecure, "", config.Internal.FrigateCookies, c.Frigate.Headers...)
		if err != nil {
			config.Internal.Status.Frigate.API = "unreachable"
			log.Warn().
				Err(err).
				Int("attempt", current_attempt).
				Int("max_tries", max_attempts).
				Int("interval", interval).
				Msgf("Cannot reach Frigate server at %v", url)
			time.Sleep(time.Duration(interval) * time.Second)
			current_attempt += 1
		} else {
			break
		}
	}
	if current_attempt == max_attempts {
		config.Internal.Status.Frigate.API = "unreachable"
		log.Fatal().
			Err(err).
			Msgf("Max attempts reached - Cannot reach Frigate server at %v. Please verify the server is running and accessible from this machine.", url)
	}
	var stats models.FrigateStats
	json.Unmarshal([]byte(response), &stats)
	log.Info().Msgf("Successfully connected to %v", url)
	config.Internal.Status.Frigate.API = "ok"
	if stats.Service.Version != "" {
		log.Debug().Msgf("Frigate server is running version %v", stats.Service.Version)
		// Save major version number
		config.Internal.FrigateVersion, _ = strconv.Atoi(strings.Split(stats.Service.Version, ".")[1])

		if config.Internal.FrigateVersion < 14 && strings.ToLower(c.App.Mode) == "reviews" {
			log.Fatal().
				Msg("Frigate must be version 0.14 or higher to use 'reviews' mode. Please use 'events' mode or update Frigate.")
		}
	}
}

func GetEventOrReview(eventOrReviewId string) ([]byte, error) {
	var c = config.ConfigData
	var appmode = strings.ToLower(c.App.Mode)
	var uri = GetEventOrReviewUri()

	url := fmt.Sprintf("%s%s/%s", c.Frigate.Server, uri, eventOrReviewId)

	var err = LoginToFrigateIfRequired(false)
	if err != nil {
		return nil, err
	}

	response, err := util.HTTPGet(url, c.Frigate.Insecure, "", config.Internal.FrigateCookies, c.Frigate.Headers...)
	if err != nil {
		config.Internal.Status.Health = "frigate webapi unreachable"
		config.Internal.Status.Frigate.API = "unreachable"
		log.Error().
			Err(err).
			Msgf("Cannot get %s from %s", appmode, url)
	} else {
		config.Internal.Status.Health = "ok"
		config.Internal.Status.Frigate.API = "ok"
	}
	return response, err
}

func GetEventsOrReviews(lastQueryTime float64) ([]byte, error) {
	var c = config.ConfigData
	var appmode = strings.ToLower(c.App.Mode)
	var params = GetQueryStringParams(lastQueryTime)
	var uri = GetEventOrReviewUri()

	url := config.ConfigData.Frigate.Server + uri + params
	log.Debug().Msgf("Checking for new %s...", appmode)

	var err = LoginToFrigateIfRequired(false)
	if err != nil {
		return nil, err
	}

	// Query API for reviews or events
	response, err := util.HTTPGet(url, c.Frigate.Insecure, "", config.Internal.FrigateCookies, c.Frigate.Headers...)
	if err != nil {
		config.Internal.Status.Health = "frigate webapi unreachable"
		config.Internal.Status.Frigate.API = "unreachable"
		log.Error().
			Err(err).
			Msgf("Cannot get %s from %s", appmode, url)
	} else {
		config.Internal.Status.Health = "ok"
		config.Internal.Status.Frigate.API = "ok"
	}

	return response, err
}

// todo mark private
func GetQueryStringParams(lastQueryTime float64) string {
	var c = config.ConfigData

	if c.Frigate.WebAPI.TestMode {
		// For testing, pull 1 event immediately
		return "?include_thumbnails=0&limit=1"
	} else {
		// Check for any events after last query time
		return "?include_thumbnails=0&after=" + strconv.FormatFloat(lastQueryTime, 'f', 6, 64)
	}
}

func GetEventOrReviewUri() string {
	var c = config.ConfigData
	var appmode = strings.ToLower(c.App.Mode)

	if appmode == "reviews" {
		return "/api/review"
	} else {
		return "/api/events"
	}
}

func LoginToFrigateIfRequired(haltOnError bool) error {
	var responseCookies []*http.Cookie
	var err error
	var c = config.ConfigData

	// Check if we need to login to Frigate
	if c.Frigate.AuthEnabled {

		//testing if we can request the /api url with status code 200
		apiUrl := fmt.Sprintf("%s/api/", c.Frigate.Server)
		_, err = util.HTTPGet(apiUrl, c.Frigate.Insecure, "", config.Internal.FrigateCookies, c.Frigate.Headers...)
		if err != nil {
			// Either we never logged in, or the bearer token expired
			// The bearer token expires after ~24h in my tests, if its expired, httpGet to /api will return a 401
			log.Warn().Err(err).Msgf("Failed to access Frigate API at %v. We will attempt to login...", apiUrl)

			loginAPIUrl := fmt.Sprintf("%s/api/login", c.Frigate.Server)
			_, responseCookies, err = util.HTTPPost(loginAPIUrl, c.Frigate.Insecure, []byte(fmt.Sprintf("{\"user\": \"%s\" , \"password\": \"%s\"}", c.Frigate.Username, c.Frigate.Password)), "", c.Frigate.Headers...)
			if err != nil {
				if haltOnError {
					log.Fatal().Err(err).Msgf("Failed to login to Frigate at %v. Please fix configuration or check if Frigate is running and accessible from this machine.", loginAPIUrl)
				} else {
					log.Error().Err(err).Msgf("Failed to login to Frigate at %v. Check if Frigate is running and accessible from this machine.", loginAPIUrl)
					return err
				}
			}

			if len(responseCookies) > 0 {
				log.Trace().Msg("Found cookie(s) in HTTP response")
				for _, cookie := range responseCookies {
					log.Trace().Msgf("Cookie: %v", cookie)
				}
			}

			if len(responseCookies) > 0 && responseCookies[0].Name == "frigate_token" {
				log.Trace().Msg("Found cookie with name 'frigate_token'")
				frigate_token := responseCookies[0].Value

				//Instead of using cookies, We could have added Authorization header with Bearer token but it might conflict with existing headers
				config.Internal.FrigateCookies = []*http.Cookie{
					{
						Name:  "frigate_token",
						Value: frigate_token,
					},
				}
				return nil
			} else {
				var err = errors.New("No cookie 'frigate_token' found in HTTP response")
				log.Error().Err(err)
				return err
			}
		} else {
			log.Trace().Msg("Already authenticated")
			return nil
		}
	} else {
		log.Trace().Msg("No authentication required")
		return nil
	}
}
