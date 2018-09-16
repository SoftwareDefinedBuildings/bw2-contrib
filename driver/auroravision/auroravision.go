package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
)

const urlBase = "https://api.auroravision.net"
const tokenRefreshPeriod = 1 * time.Hour

type AuroraVision struct {
	UserID            string
	Password          string
	APIKey            string
	PlantID           int32
	sessionToken      string
	lastAuthenticated time.Time
	req               *gorequest.SuperAgent
}

type authResponse struct {
	Token string `json:"result"`
}

type plantBilling struct {
	EnergyToday    float64 `json:"plantCurrentDayEnergykWh,string"`
	EnergyLifetime float64 `json:"plantLifetimeEnergykWh,string"`
}

type plantBillingWrapper struct {
	Result plantBilling `json:"result"`
}

type powerMeasurement struct {
	StartTime int64   `json:"start"`
	EndTime   int64   `json:"end"`
	Value     float64 `json:"value"`
}

type powerTimeseriesWrapper struct {
	Result []powerMeasurement `json:"result"`
}

type Summary struct {
	CurrentPower   float64
	EnergyToday    float64
	EnergyLifetime float64
}

func NewAuroraVision(userID, password, apiKey string, plantID int32) *AuroraVision {
	return &AuroraVision{
		UserID:   userID,
		Password: password,
		APIKey:   apiKey,
		PlantID:  plantID,
		req:      gorequest.New(),
	}
}

func (av *AuroraVision) authenticate() error {
	resp, body, errs := av.req.Get(urlBase+"/api/rest/authenticate").
		AppendHeader("X-AuroraVision-ApiKey", av.APIKey).
		SetBasicAuth(av.UserID, av.Password).
		EndBytes()
	if errs != nil {
		return fmt.Errorf("Authentication Error: %+v", errs)
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("Authentication Error: Code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	var authResp authResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return errors.Wrap(err, "Failed to parse authentication response")
	}

	av.sessionToken = authResp.Token
	return nil
}

func (av *AuroraVision) ReadSummary() (*Summary, error) {
	// Periodically refresh authentication token to avoid expiration
	if time.Now().Sub(av.lastAuthenticated) > tokenRefreshPeriod {
		if err := av.authenticate(); err != nil {
			return nil, errors.Wrap(err, "Could not refresh session token")
		}
		av.lastAuthenticated = time.Now()
	}

	url := fmt.Sprintf("%s/api/rest/v1/plant/%d/billingData", urlBase, av.PlantID)
	resp, body, errs := av.req.Get(url).
		AppendHeader("X-AuroraVision-Token", av.sessionToken).
		EndBytes()
	if errs != nil {
		return nil, fmt.Errorf("Could not retrieve plant billing data: %+v", errs)
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Could not retrieve plant billing data: Error Code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	var billingResp plantBillingWrapper
	if err := json.Unmarshal(body, &billingResp); err != nil {
		return nil, errors.Wrap(err, "Failed to parse plant billing response")
	}

	// We can't get current power data directly
	// Instead we need to extract it from a timeseries
	url = fmt.Sprintf("%s/api/rest/v1/stats/power/timeseries/%d/GenerationPower/average", urlBase, av.PlantID)
	startDate := time.Now().UTC().Add(-24 * time.Hour).Format("20060102")
	endDate := time.Now().UTC().Format("20060102")
	resp, body, errs = av.req.Get(url).
		Param("startDate", startDate).
		Param("endDate", endDate).
		Param("sampleSize", "Min5").
		Param("timeZone", "UTC").
		AppendHeader("X-AuroraVision-Token", av.sessionToken).
		EndBytes()
	if errs != nil {
		return nil, fmt.Errorf("Could not retrieve plant power data: %+v", errs)
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Could not retrieve plant power data: Error Code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	var powerResp powerTimeseriesWrapper
	if err := json.Unmarshal(body, &powerResp); err != nil {
		return nil, errors.Wrap(err, "Failed to parse plant power data response")
	}

	sort.Slice(powerResp.Result, func(i, j int) bool { return powerResp.Result[i].StartTime < powerResp.Result[j].StartTime })
	return &Summary{
		// API reports power in W and energy in kWh
		CurrentPower:   powerResp.Result[len(powerResp.Result)-1].Value,
		EnergyToday:    billingResp.Result.EnergyToday * 1000,
		EnergyLifetime: billingResp.Result.EnergyLifetime * 1000,
	}, nil
}

func (av *AuroraVision) PollSummary(context context.Context, period time.Duration) (chan *Summary, chan error) {
	summCh := make(chan *Summary)
	errCh := make(chan error, 1)

	tick := time.Tick(period)
	go func() {
		for {
			select {
			case <-context.Done():
				close(summCh)
				close(errCh)
				return

			case <-tick:
				summary, err := av.ReadSummary()
				if err != nil {
					close(summCh)
					errCh <- err
					return
				} else {
					summCh <- summary
				}
			}
		}
	}()

	return summCh, errCh
}
