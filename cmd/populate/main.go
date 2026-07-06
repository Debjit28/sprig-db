package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var token string

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

func insertDoc(collection string, payload []byte) {
	req, err := http.NewRequest("POST", "http://localhost:7777/api/"+collection, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		fmt.Printf("Inserted into %s\n", collection)
	} else {
		fmt.Println("Failed to insert:", err)
	}
}

func main() {
	fmt.Println("🚀 Getting Authenticated to Sprig-DB...")
	initAuth()

	fmt.Println("📦 Populating Users collection...")
	users := []string{
		`{"name": "Alice Smith", "email": "alice@example.com", "role": "admin", "joined": "2024-01-15"}`,
		`{"name": "Bob Jones", "email": "bob@example.com", "role": "customer", "joined": "2024-03-22"}`,
		`{"name": "Charlie Adams", "email": "charlie@example.com", "role": "customer", "joined": "2024-05-10"}`,
		`{"name": "Diana Prince", "email": "diana@example.com", "role": "vendor", "joined": "2023-11-05"}`,
	}
	for _, u := range users {
		insertDoc("users", []byte(u))
	}

	fmt.Println("🛒 Populating Products collection...")
	products := []string{
		`{"name": "Wireless Headphones", "price": 120.50, "category": "Electronics", "in_stock": true}`,
		`{"name": "Mechanical Keyboard", "price": 99.99, "category": "Electronics", "in_stock": true}`,
		`{"name": "Ergonomic Desk Chair", "price": 250.00, "category": "Furniture", "in_stock": false}`,
		`{"name": "Coffee Beans (1kg)", "price": 24.99, "category": "Groceries", "in_stock": true}`,
		`{"name": "Yoga Mat", "price": 35.00, "category": "Fitness", "in_stock": true}`,
	}
	for _, p := range products {
		insertDoc("products", []byte(p))
	}

	fmt.Println("💳 Populating Orders collection...")
	orders := []string{
		`{"user_email": "bob@example.com", "product_name": "Mechanical Keyboard", "total": 99.99, "status": "shipped"}`,
		`{"user_email": "charlie@example.com", "product_name": "Coffee Beans (1kg)", "total": 24.99, "status": "processing"}`,
		`{"user_email": "alice@example.com", "product_name": "Ergonomic Desk Chair", "total": 250.00, "status": "delivered"}`,
	}
	for _, o := range orders {
		insertDoc("orders", []byte(o))
	}

	fmt.Println("✅ Sensible data population complete!")
}
