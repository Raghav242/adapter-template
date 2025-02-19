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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	framework "github.com/sgnl-ai/adapter-framework"
	api_adapter_v1 "github.com/sgnl-ai/adapter-framework/api/adapter/v1"
)

const (
	// SCAFFOLDING #11 - pkg/adapter/datasource.go: Update the set of valid entity types this adapter supports.
	Teams string = "teams"
)

// Entity contains entity specific information, such as the entity's unique ID attribute and the
// endpoint to query that entity.
type Entity struct {
	// SCAFFOLDING #12 - pkg/adapter/datasource.go: Update Entity fields used to store entity specific information
	// Add or remove fields as needed. This should be used to store entity specific information
	// such as the entity's unique ID attribute name and the endpoint to query that entity.

	// uniqueIDAttrExternalID is the external ID of the entity's uniqueId attribute.
	uniqueIDAttrExternalID string
}

// Datasource directly implements a Client interface to allow querying
// an external datasource.
type Datasource struct {
	Client *http.Client
}

type DatasourceResponse struct {
	// SCAFFOLDING #13  - pkg/adapter/datasource.go: Add or remove fields in the response as necessary. This is used to unmarshal the response from the SoR.

	// SCAFFOLDING #14 - pkg/adapter/datasource.go: Update `objects` with field name in the SoR response that contains the list of objects.
	Teams  []map[string]interface{} `json:"teams,omitempty"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
	Total  *int                     `json:"total,omitempty"`
	More   bool                     `json:"more"`
}

var (
	// SCAFFOLDING #15 - pkg/adapter/datasource.go: Update the set of valid entity types supported by this adapter. Used for validation.

	// ValidEntityExternalIDs is a map of valid external IDs of entities that can be queried.
	// The map value is the Entity struct which contains the unique ID attribute.
	ValidEntityExternalIDs = map[string]Entity{
		Teams: {
			uniqueIDAttrExternalID: "id",
		},
	}
)

// NewClient returns a Client to query the datasource.
func NewClient(timeout int) Client {
	return &Datasource{
		Client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (d *Datasource) GetPage(ctx context.Context, request *Request) (*Response, *framework.Error) {
	var req *http.Request

	// SCAFFOLDING #16 - pkg/adapter/datasource.go: Create the SoR API URL
	// Populate the request with the appropriate path, headers, and query parameters to query the
	// datasource.
	fullURL := request.BaseURL + "/" + request.EntityExternalID // Join the base URL and path

	url, err := url.Parse(fullURL) // Now parse the *combined* URL
	if err != nil {
		return nil, &framework.Error{
			Message: fmt.Sprintf("Failed to parse URL: %v", err), // Include the error for debugging
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	q := url.Query()
	pageSize := int(request.PageSize)
	if pageSize > 0 {
		q.Add("limit", fmt.Sprintf("%d", pageSize))
	}
	if request.Cursor != "" {
		q.Add("offset", request.Cursor)
	}
	url.RawQuery = q.Encode()

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to create HTTP request to datasource.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// Timeout API calls that take longer than 5 seconds
	apiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req = req.WithContext(apiCtx)

	// SCAFFOLDING #17 - pkg/adapter/datasource.go: Add any headers required to communicate with the SoR APIs.
	req.Header.Add("Accept", "application/vnd.pagerduty+json;version=2")
	req.Header.Add("Content-Type", "application/json")

	if request.Token == "" {
		return nil, &framework.Error{
			Message: "PagerDuty auth is missing required token.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INVALID_DATASOURCE_CONFIG,
		}
	} else {
		req.Header.Add("Authorization", "Token token="+request.Token) // Correctly use the token from request.Token
	}

	// Sending the request
	res, err := d.Client.Do(req)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to send request to datasource.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// Read and unmarshal response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to read response body.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// Deserialize JSON into the datastructure
	var response DatasourceResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, &framework.Error{
			Message: fmt.Sprintf("Failed to deserialize response body: %v", err),
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// Check the 'X-Next-Page' header for pagination
	cursor := ""
	if res.Header.Get("X-Next-Page") != "" {
		cursor = res.Header.Get("X-Next-Page")
	} else {
		// If there's no cursor, set cursor to empty string to indicate the end of pagination
		cursor = ""
	}

	// Return a valid response containing the objects and cursor
	return &Response{
		Objects: response.Teams, // Parse the teams
		Cursor:  cursor,         // Handle cursor if provided in the header
	}, nil
}
