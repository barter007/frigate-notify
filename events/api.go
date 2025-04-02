package events

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x2142/frigate-notify/config"
	"github.com/0x2142/frigate-notify/frigate"
	"github.com/0x2142/frigate-notify/models"
)

// LastQueryTime tracks the timestamp of the last event seen
var LastQueryTime float64 = float64(time.Now().Unix())

// QueryAPI queries the Frigate API for new events or reviews
func QueryAPI() {
	appmode := strings.ToLower(config.ConfigData.App.Mode)

	response, err := frigate.GetEventsOrReviews(LastQueryTime)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting %s", appmode)
		return
	}

	switch appmode {
	case "reviews":
		var reviews []models.Review
		json.Unmarshal([]byte(response), &reviews)
		log.Debug().Msgf("Found %v new reviews", len(reviews))

		for _, review := range reviews {
			// Update last event check time with most recent timestamp
			if review.StartTime > LastQueryTime {
				LastQueryTime = review.StartTime
			}
			processReview(review)
		}
	case "events":
		var events []models.Event
		json.Unmarshal([]byte(response), &events)
		log.Debug().Msgf("Found %v new events", len(events))
		for _, event := range events {
			// Copy zones to CurrentZones, which is used for filters
			event.CurrentZones = event.Zones
			// Update last event check time with most recent timestamp
			if event.StartTime > LastQueryTime {
				LastQueryTime = event.StartTime
			}
			processEvent(event)
		}
	}
}
