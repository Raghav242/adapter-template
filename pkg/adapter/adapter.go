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
	"time"

	framework "github.com/sgnl-ai/adapter-framework"
	api_adapter_v1 "github.com/sgnl-ai/adapter-framework/api/adapter/v1"
	"github.com/sgnl-ai/adapter-framework/web"
)

// Adapter implements the framework.Adapter interface to query pages of objects
// from datasources.
type Adapter struct {
	// SCAFFOLDING #20 - pkg/adapter/adapter.go: Add or remove fields to configure the adapter.

	// Client provides access to the datasource.
	Client Client
}

// NewAdapter instantiates a new Adapter.
//
// SCAFFOLDING #21 - pkg/adapter/adapter.go: Add or remove parameters to match field updates above.
func NewAdapter(client Client) framework.Adapter[Config] {
	return &Adapter{
		Client: client,
	}
}

// GetPage is called by SGNL's ingestion service to query a page of objects
// from a datasource.
func (a *Adapter) GetPage(ctx context.Context, request *framework.Request[Config]) framework.Response {
	if err := a.ValidateGetPageRequest(ctx, request); err != nil {
		return framework.NewGetPageResponseError(err)
	}

	return a.RequestPageFromDatasource(ctx, request)
}

// RequestPageFromDatasource requests a page of objects from a datasource.
func (a *Adapter) RequestPageFromDatasource(
	ctx context.Context, request *framework.Request[Config],
) framework.Response {

	// SCAFFOLDING #22 - pkg/adapter/adapter.go: Modify implementation to query your SoR.
	// If necessary, update this entire method to query your SoR. All of the code in this function
	// can be updated to match your SoR requirements.

	apiURL := "https://api.pagerduty.com/teams"

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return framework.NewGetPageResponseError(
			&framework.Error{
				Message: fmt.Sprintf("Failed to create request: %v", err),
				Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
			},
		)
	}

	// Set required headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token token="+request.Auth.HTTPAuthorization)

	// Perform the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return framework.NewGetPageResponseError(
			&framework.Error{
				Message: fmt.Sprintf("Failed to perform request: %v", err),
				Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
			},
		)
	}
	defer resp.Body.Close()

	// An adapter error message is generated if the response status code is not
	// successful (i.e. if not statusCode >= 200 && statusCode < 300).
	if adapterErr := web.HTTPError(resp.StatusCode, resp.Header.Get("Retry-After")); adapterErr != nil {
		return framework.NewGetPageResponseError(adapterErr)
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return framework.NewGetPageResponseError(
			&framework.Error{
				Message: fmt.Sprintf("Failed to read response body: %v", err),
				Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
			},
		)
	}

	// Parse JSON into DatasourceResponse
	var data DatasourceResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return framework.NewGetPageResponseError(
			&framework.Error{
				Message: fmt.Sprintf("Failed to unmarshal JSON response: %v", err),
				Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
			},
		)
	}

	// Use data.Teams instead of jsonData
	parsedObjects, parserErr := web.ConvertJSONObjectList(
		&request.Entity,
		data.Teams, // Updated: Use Teams from the DatasourceResponse

		// SCAFFOLDING #23 - pkg/adapter/adapter.go: Disable JSONPathAttributeNames.
		// Disable JSONPathAttributeNames if your datasource does not support
		// JSONPath attribute names. This should be enabled for most datasources.
		web.WithJSONPathAttributeNames(),

		// SCAFFOLDING #24 - pkg/adapter/adapter.go: List datetime formats supported by your SoR.
		// Provide a list of datetime formats supported by your datasource if
		// they are known. This will optimize the parsing of datetime values.
		// If this is not known, you can omit this option which will try
		// a list of common datetime formats.
		web.WithDateTimeFormats(
			[]web.DateTimeFormatWithTimeZone{
				{Format: time.RFC3339, HasTimeZone: true},
				{Format: time.RFC3339Nano, HasTimeZone: true},
				{Format: "2006-01-02T15:04:05.000Z0700", HasTimeZone: true},
				{Format: "2006-01-02", HasTimeZone: false},
			}...,
		),
	)
	if parserErr != nil {
		return framework.NewGetPageResponseError(
			&framework.Error{
				Message: fmt.Sprintf("Failed to convert datasource response objects: %v.", parserErr),
				Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
			},
		)
	}

	page := &framework.Page{
		Objects: parsedObjects,
	}

	return framework.NewGetPageResponseSuccess(page)
}
