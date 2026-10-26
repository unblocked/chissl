package utils

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func HttpRequestWithBodyWithBasicAuth(method, url, body, username, password string) (string, error) {
	// Create a new request with the given URL and body
	req, err := http.NewRequest(strings.ToUpper(method), url, strings.NewReader(body))
	if err != nil {
		return "", err
	}

	// Set the Basic Auth header
	auth := username + ":" + password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	// Create a custom http.Client with the Basic Auth header
	client := &http.Client{}

	// Perform the request using the custom client
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Check if the status code is not in the 2xx range
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(b), fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	return string(b), err
}

func HttpRequestNoBodyWithBasicAuth(method, url, username, password string) (string, error) {
	// Create a new request
	req, err := http.NewRequest(strings.ToUpper(method), url, nil)
	if err != nil {
		return "", err
	}

	// Set the Basic Auth header
	auth := username + ":" + password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Check if the status code is not in the 2xx range
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(b), fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return string(b), nil
}
