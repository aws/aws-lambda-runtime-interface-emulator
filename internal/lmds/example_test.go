// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package lmds_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lmds"
)

func Example() {

	type extendedMetadata struct {
		lmds.Metadata
		CapacityProviderArn string `json:"CapacityProviderArn"`
	}

	service := lmds.NewService("test-token")

	metadata := &extendedMetadata{
		Metadata: lmds.Metadata{
			AvailabilityZoneID: "use1-az1",
		},
		CapacityProviderArn: "arn:aws:ecs:us-east-1:123456789012:capacity-provider/my-capacity-provider",
	}
	metadataBytes, _ := json.Marshal(metadata)
	service.UpdateMetadata(lmds.MetadataConfig{
		Data:   metadataBytes,
		MaxAge: 1 * time.Second,
	})

	server := httptest.NewServer(service)
	defer server.Close()

	updatedMetadata := &extendedMetadata{
		Metadata: lmds.Metadata{
			AvailabilityZoneID: "use1-az3",
		},
		CapacityProviderArn: "arn:aws:ecs:us-east-1:123456789012:capacity-provider/another-provider",
	}
	updatedMetadataBytes, _ := json.Marshal(updatedMetadata)
	service.UpdateMetadata(lmds.MetadataConfig{
		Data:   updatedMetadataBytes,
		MaxAge: 12 * time.Hour,
	})

	metrics := service.Metrics.Take()
	fmt.Printf("Metrics: clientErrs=%d serverErrs=%d successfulCalls=%d\n",
		metrics.ClientErrors, metrics.ServerErrors, metrics.SuccessfulCalls)

}

func Example_client() {

	metadata := lmds.Metadata{
		AvailabilityZoneID: "use1-az1",
	}
	metadataBytes, _ := json.Marshal(metadata)
	const TOKEN = "test-secret-token"
	service := lmds.NewService(TOKEN)
	service.UpdateMetadata(lmds.MetadataConfig{
		Data:   metadataBytes,
		MaxAge: 12 * time.Hour,
	})
	server := httptest.NewServer(service)
	defer server.Close()

	_ = os.Setenv("AWS_LAMBDA_METADATA_API", server.Listener.Addr().String())
	_ = os.Setenv("AWS_LAMBDA_METADATA_TOKEN", TOKEN)

	client := &http.Client{}
	endpoint := fmt.Sprintf("http://%s/2026-01-15/metadata/execution-environment", os.Getenv("AWS_LAMBDA_METADATA_API"))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("AWS_LAMBDA_METADATA_TOKEN"))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Unexpected status code: %d\n", resp.StatusCode)
		return
	}

	cacheControl := resp.Header.Get("Cache-Control")
	var maxAge time.Duration
	for _, directive := range strings.Split(cacheControl, ",") {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "max-age=") {
			if seconds, err := strconv.Atoi(strings.TrimPrefix(directive, "max-age=")); err == nil {
				maxAge = time.Duration(seconds) * time.Second
			}
		}
	}
	fmt.Printf("MaxAge: %v\n", maxAge)

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error decoding response: %v\n", err)
		return
	}

	fmt.Printf("AvailabilityZoneID: %s\n", result["AvailabilityZoneID"])

}
