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

func getJWT() string {
	payload := []byte(`{"username":"loadtester","password":"password123"}`)
	// try register first
	http.Post("http://localhost:7777/auth/register", "application/json", bytes.NewBuffer(payload))
	// then login
	resp, err := http.Post("http://localhost:7777/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	token, ok := data["token"].(string)
	if !ok {
		fmt.Println("Error:", string(body))
		panic("Could not get JWT token")
	}
	return token
}

func main() {
	fmt.Println("🚀 Starting Sprig-DB API Load Test...")
	fmt.Println("Authenticating and getting JWT token...")
	
	token := getJWT()

	fmt.Println("Blasting the server with HTTP POST requests via multiple goroutines...")

	totalRequests := 25000 // A massive amount to demonstrate capability
	concurrentWorkers := 200 // Number of simultaneous clients
	
	var successful int32
	var failed int32

	start := time.Now()
	
	var wg sync.WaitGroup
	requestsPerWorker := totalRequests / concurrentWorkers
	
	payload := []byte(`{"tester": "agent", "type": "throughput_validation", "complexity": "low"}`)

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			t := http.DefaultTransport.(*http.Transport).Clone()
			t.MaxIdleConns = 100
			t.MaxConnsPerHost = 100
			t.MaxIdleConnsPerHost = 100

			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: t,
			}

			for j := 0; j < requestsPerWorker; j++ {
				req, err := http.NewRequest("POST", "http://localhost:7777/api/loadtest_metrics", bytes.NewBuffer(payload))
				if err != nil {
					atomic.AddInt32(&failed, 1)
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				
				resp, err := client.Do(req)
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
	fmt.Printf("Total Time         : %.2fs\n", duration.Seconds())
	fmt.Printf("Total API Requests : %d\n", totalRequests)
	fmt.Printf("Successful Requests: %d\n", successful)
	fmt.Printf("Failed Requests    : %d\n", failed)
	fmt.Printf("HTTP Requests/sec  : %.2f req/s\n", float64(totalRequests)/duration.Seconds())
	fmt.Println("-------------------------------------")
}
