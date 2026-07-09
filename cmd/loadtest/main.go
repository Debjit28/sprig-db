package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL    = "http://localhost:7777"
	collection = "loadtest_metrics"
	username   = "loadtester"
	password   = "password123"
)

func main() {
	fmt.Println("🚀 Starting Sprig-DB API Load Test (schema-validated inserts)...")

	token := getJWT()
	client := newHTTPClient()

	fmt.Println("📋 Creating collection schema for load test...")
	if err := setupCollection(client, token); err != nil {
		panic(err)
	}

	fmt.Println("🧪 Running schema validation smoke test...")
	if err := runValidationSmokeTest(client, token); err != nil {
		panic(err)
	}

	fmt.Println("🔥 Blasting the server with schema-valid POST requests...")

	totalRequests := 25000
	concurrentWorkers := 200

	var successful int32
	var failed int32

	start := time.Now()

	var wg sync.WaitGroup
	requestsPerWorker := totalRequests / concurrentWorkers

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			workerClient := newHTTPClient()
			complexities := []string{"low", "medium", "high"}

			for j := 0; j < requestsPerWorker; j++ {
				idx := workerID*requestsPerWorker + j
				payload := map[string]any{
					"tester":     fmt.Sprintf("worker-%d-%d", workerID, idx),
					"type":       "throughput_validation",
					"complexity": complexities[idx%len(complexities)],
					"score":      float64(idx % 10000),
				}
				body, err := json.Marshal(payload)
				if err != nil {
					atomic.AddInt32(&failed, 1)
					continue
				}

				req, err := http.NewRequest(
					"POST",
					baseURL+"/api/records/"+collection,
					bytes.NewBuffer(body),
				)
				if err != nil {
					atomic.AddInt32(&failed, 1)
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := workerClient.Do(req)
				if err != nil {
					atomic.AddInt32(&failed, 1)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					atomic.AddInt32(&successful, 1)
				} else {
					atomic.AddInt32(&failed, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Println("\n📊 --- API HTTP Load Test Results ---")
	fmt.Printf("Collection           : %s\n", collection)
	fmt.Printf("Total Time           : %.2fs\n", duration.Seconds())
	fmt.Printf("Total API Requests   : %d\n", totalRequests)
	fmt.Printf("Successful Requests  : %d\n", successful)
	fmt.Printf("Failed Requests      : %d\n", failed)
	fmt.Printf("HTTP Requests/sec    : %.2f req/s\n", float64(totalRequests)/duration.Seconds())
	fmt.Println("-------------------------------------")
}

func getJWT() string {
	payload := []byte(fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password))
	http.Post(baseURL+"/auth/register", "application/json", bytes.NewBuffer(payload))

	resp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		panic(err)
	}
	token, ok := data["token"].(string)
	if !ok {
		fmt.Println("Error:", string(body))
		panic("could not get JWT token")
	}
	return token
}

func setupCollection(client *http.Client, token string) error {
	schema := map[string]any{
		"name": collection,
		"fields": map[string]any{
			"tester": map[string]any{
				"type":      "string",
				"required":  true,
				"minLength": 2,
				"maxLength": 64,
			},
			"type": map[string]any{
				"type":     "string",
				"required": true,
				"enum":     []string{"throughput_validation", "stress"},
			},
			"complexity": map[string]any{
				"type":     "string",
				"required": true,
				"enum":     []string{"low", "medium", "high"},
			},
			"score": map[string]any{
				"type":     "number",
				"required": true,
				"min":      0,
				"max":      10000,
			},
		},
	}

	body, err := json.Marshal(schema)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", baseURL+"/api/collections", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create collection schema (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func runValidationSmokeTest(client *http.Client, token string) error {
	validPayload := []byte(`{"tester":"smoke-test","type":"throughput_validation","complexity":"low","score":1}`)
	if status, body, err := postRecord(client, token, validPayload); err != nil {
		return err
	} else if status != http.StatusCreated {
		return fmt.Errorf("expected valid insert to succeed, got %d: %s", status, body)
	}

	invalidCases := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "missing required field",
			payload: []byte(`{"tester":"smoke-test","type":"throughput_validation","complexity":"low"}`),
		},
		{
			name:    "unknown field",
			payload: []byte(`{"tester":"smoke-test","type":"throughput_validation","complexity":"low","score":1,"extra":"bad"}`),
		},
		{
			name:    "invalid enum",
			payload: []byte(`{"tester":"smoke-test","type":"not_allowed","complexity":"low","score":1}`),
		},
		{
			name:    "wrong type",
			payload: []byte(`{"tester":"smoke-test","type":"throughput_validation","complexity":"low","score":"nope"}`),
		},
	}

	for _, tc := range invalidCases {
		status, body, err := postRecord(client, token, tc.payload)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("expected %q to be rejected with 400, got %d: %s", tc.name, status, body)
		}
		fmt.Printf("  ✓ rejected invalid payload: %s\n", tc.name)
	}

	return nil
}

func postRecord(client *http.Client, token string, payload []byte) (int, string, error) {
	req, err := http.NewRequest("POST", baseURL+"/api/records/"+collection, bytes.NewBuffer(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), nil
}

func newHTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: t,
	}
}
