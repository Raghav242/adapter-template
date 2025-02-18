// Copyright 2023 SGNL.ai, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"log"
)

// Config is the optional configuration passed in each GetPage call to the adapter.
type Config struct {
	// SCAFFOLDING #3 - pkg/adapter/config.go - pass Adapter config fields.
	// Every field MUST have a `json` tag.

	// API version to be used in requests.
	APIVersion string `json:"apiVersion,omitempty"`

	// API base URL for the external system.
	APIBaseURL string `json:"apiBaseURL,omitempty"`

	// Authorization token for API requests.
	AuthToken string `json:"authToken,omitempty"`

	// Accept header for API requests.
	AcceptHeader string `json:"acceptHeader,omitempty"`

	// Content-Type header for API requests.
	ContentType string `json:"contentType,omitempty"`
}

// ValidateConfig validates that a Config received in a GetPage call is valid.
func (c *Config) Validate(_ context.Context) error {
	// SCAFFOLDING #4 - pkg/adapter/config.go: Validate fields passed in Adapter config.
	// Update the checks below to validate the fields in Config.

	// Log the received config for debugging purposes.
	log.Printf("Validating config: %+v", c)

	switch {
	case c == nil:
		return errors.New("request contains no config")
	case c.APIVersion == "":
		return errors.New("apiVersion is not set")
	case c.APIBaseURL == "":
		return errors.New("apiBaseURL is not set")
	case c.AuthToken == "":
		return errors.New("authToken is not set")
	case c.AcceptHeader == "":
		return errors.New("acceptHeader is not set")
	case c.ContentType == "":
		return errors.New("contentType is not set")
	default:
		return nil
	}
}

// ParseConfig parses the JSON string from the gRPC request into a Config struct.
func ParseConfig(configStr string) (*Config, error) {
	var config Config
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, errors.New("failed to parse config JSON: " + err.Error())
	}
	return &config, nil
}
