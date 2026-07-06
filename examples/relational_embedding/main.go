package main

import (
	"fmt"
	"github.com/Debjit28/sprig-db/sprig"
)

// This example demonstrates how you can natively integrate Sprig-DB 
// into your Go program and build a Relational Model on top of its Document Store.

func main() {
	fmt.Println("🌱 Demonstrating Relational Integration with Sprig-DB...")

	// 1. Initialize embedded DB natively in your Go program
	db, err := sprig.New(sprig.WithDBName("relational_demo"))
	if err != nil {
		panic(err)
	}
	// Clean up after the demo finishes
	defer db.DropDatabase("relational_demo")

	// ---------------------------------------------------------
	// 2. Insert into "users" (The Parent Table)
	// ---------------------------------------------------------
	userID, _ := db.Coll("users").Insert(sprig.Map{
		"username": "janedoe",
		"email":    "jane@example.com",
	})
	
	fmt.Printf("Created User! Assigned ID: %d\n", userID)

	// ---------------------------------------------------------
	// 3. Insert into "orders" (The Child Table with a Foreign Key)
	// ---------------------------------------------------------
	db.Coll("orders").Insert(sprig.Map{
		"user_id": userID, // FOREIGN KEY relation mapping back to the "users" collection
		"item":    "Mechanical Keyboard",
		"price":   99.99,
	})
	db.Coll("orders").Insert(sprig.Map{
		"user_id": userID, // FOREIGN KEY
		"item":    "Go Programming Book",
		"price":   35.50,
	})

	// ---------------------------------------------------------
	// 4. Perform an Application-Level Relational JOIN
	// ---------------------------------------------------------
	fmt.Println("\n---- Fetching User and their Orders (JOIN) ----")

	// Step A: Fetch the parent record
	userResult, _ := db.Coll("users").Eq(sprig.Map{"id": userID}).Find()
	if len(userResult.Data) == 0 {
		fmt.Println("User not found.")
		return
	}
	user := userResult.Data[0]
	
	// Step B: Fetch the associated child records natively using the Eq filter
	// Note: Because it's stored and retrieved as JSON internally, we cast the ID to float64 for equality filtering
	orderResult, _ := db.Coll("orders").Eq(sprig.Map{"user_id": float64(userID)}).Find()
	
	// Output the relational data
	fmt.Printf("User Profile: %s | Contact: %s\n", user["username"], user["email"])
	fmt.Printf("Total Orders Found: %d\n", orderResult.Total)
	
	var totalSpent float64
	for _, order := range orderResult.Data {
		price := order["price"].(float64)
		fmt.Printf(" - %s ($%.2f)\n", order["item"], price)
		totalSpent += price
	}
	
	fmt.Printf("Total Spent: $%.2f\n", totalSpent)
}
