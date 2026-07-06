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

var token string
var failed int32
var success int32

func initAuth() {
	payload := []byte(`{"username":"admin_populator", "password":"password123"}`)
	http.Post("http://localhost:7777/auth/register", "application/json", bytes.NewBuffer(payload))
	resp, err := http.Post("http://localhost:7777/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	token = data["token"].(string)
}

func massiveInsert(collection string, count int) {
	fmt.Printf("📦 Inserting %d documents into %s (Please wait ~60s)...\n", count, collection)

	workers := 100
	reqsPerWorker := count / workers
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			t := http.DefaultTransport.(*http.Transport).Clone()
			t.MaxIdleConns = 100
			t.MaxConnsPerHost = 100
			client := &http.Client{Timeout: 15 * time.Second, Transport: t}

			for j := 0; j < reqsPerWorker; j++ {
				idx := (workerID * reqsPerWorker) + j
				
				// Format payload to make it unique and filterable
				var finalPayload []byte
				if collection == "users" {
					role := "customer"
					if idx%10 == 0 { role = "admin" }
					finalPayload = []byte(fmt.Sprintf(`{"name": "User%d", "role": "%s", "active": "true"}`, idx, role))
				} else if collection == "products" {
					category := "Electronics"
					if idx%5 == 0 { category = "Furniture" }
					finalPayload = []byte(fmt.Sprintf(`{"product": "Item%d", "price": %.2f, "category": "%s"}`, idx, float64(idx)*1.5, category))
				} else {
					status := "processing"
					if idx%3 == 0 { status = "shipped" }
					finalPayload = []byte(fmt.Sprintf(`{"order_id": "ORD-%d", "status": "%s"}`, idx, status))
				}

				req, _ := http.NewRequest("POST", "http://localhost:7777/api/"+collection, bytes.NewBuffer(finalPayload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				if err != nil {
					atomic.AddInt32(&failed, 1)
					continue
				}
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					atomic.AddInt32(&success, 1)
				} else {
					atomic.AddInt32(&failed, 1)
				}
			}
		}(i)
	}
	wg.Wait()
}

func main() {
	fmt.Println("🚀 Getting Authenticated to Sprig-DB...")
	initAuth()

	start := time.Now()
	// Total exactly 31,000 docs
	massiveInsert("users", 10000)
	massiveInsert("products", 10000)
	massiveInsert("orders", 11000)

	fmt.Printf("\n✅ Sensible 31k data population complete in %v!\n", time.Since(start))
	fmt.Printf("Total Success: %d | Total Failed: %d\n", success, failed)
}
