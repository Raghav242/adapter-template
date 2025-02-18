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
	"strconv"
	"time"

	framework "github.com/sgnl-ai/adapter-framework"
	api_adapter_v1 "github.com/sgnl-ai/adapter-framework/api/adapter/v1"
)

const (
	// SCAFFOLDING #11 - pkg/adapter/datasource.go: Update the set of valid entity types this adapter supports.
	// Users  string = "users"
	// Groups string = "groups"
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
	//Objects []map[string]any `json:"objects,omitempty"`
	Teams []map[string]interface{} `json:"teams,omitempty"`
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
	// Add headers to the request, if any.
	// req.Header.Add("Accept", "application/json")

	req.Header.Add("Accept", "application/vnd.pagerduty+json;version=2")
	req.Header.Add("Content-Type", "application/json")

	if request.Token == "" {
		return nil, &framework.Error{ // Return an error if no token is provided
			Message: "PagerDuty auth is missing required token.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INVALID_DATASOURCE_CONFIG,
		}
	} else {
		// Auth Token for PagerDuty API (using http_authorization from gRPC request)
		req.Header.Add("Authorization", request.Token) // Correctly use the token from request.Token
	}

	res, err := d.Client.Do(req)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to send request to datasource.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	response := &Response{
		StatusCode:       res.StatusCode,
		RetryAfterHeader: res.Header.Get("Retry-After"),
	}

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body) // Read response body for better error messages
		defer res.Body.Close()
		return response, &framework.Error{
			Message: fmt.Sprintf("PagerDuty API request failed: %d - %s", res.StatusCode, string(body)),
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_DATASOURCE_FAILED,
		}
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to read response body.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_DATASOURCE_FAILED,
		}
	}
	// SCAFFOLDING #17-1 - pkg/adapter/datasource.go: To add support for multiple entities that require different parsing functions
	// Add code to call different ParseResponse functions for each entity response.
	var objects []map[string]interface{}
	var nextCursor string
	var parseErr *framework.Error

	switch request.EntityExternalID {
	case Teams:
		objects, nextCursor, parseErr = ParseResponse(body, int(request.PageSize), request.Cursor) // Pass pageSize and cursor
	default:
		return nil, &framework.Error{
			Message: fmt.Sprintf("Unsupported entity type: %s", request.EntityExternalID),
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INVALID_ENTITY_CONFIG,
		}
	}

	if parseErr != nil {
		return nil, parseErr
	}

	response.Objects = objects
	response.NextCursor = nextCursor

	return response, nil
}

func ParseResponse(body []byte, pageSize int, cursor string) (objects []map[string]interface{}, nextCursor string, err *framework.Error) {
	var data DatasourceResponse // No need for pointer here

	unmarshalErr := json.Unmarshal(body, &data)
	if unmarshalErr != nil {
		return nil, "", &framework.Error{
			Message: fmt.Sprintf("Failed to unmarshal the datasource response: %v.", unmarshalErr),
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// SCAFFOLDING #18 - pkg/adapter/datasource.go: Add response validations.
	// Add necessary validations to check if the response from the datasource is what is expected.

	if data.Teams == nil { // Example Validation: Check if "teams" is present
		return nil, "", &framework.Error{
			Message: "PagerDuty API response is missing 'teams' field.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_DATASOURCE_FAILED,
		}
	}

	// SCAFFOLDING #19 - pkg/adapter/datasource.go: Populate next page information (called cursor in SGNL adapters).
	// Populate nextCursor with the cursor returned from the datasource, if present.

	if len(data.Teams) == pageSize {
		offset, _ := strconv.Atoi(cursor)
		nextCursor = strconv.Itoa(offset + pageSize)
	} else {
		nextCursor = ""
	}

	return data.Teams, nextCursor, nil
}
