package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func main() {
	dbURL := "libsql://org-final-turso-org-33586-ahmedhamid1242.aws-us-east-2.turso.io"
	token := "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE3Njk2MzM1ODcsImlkIjoiOGI5NzQzNzUtMzVmYi00MzMxLTgzYzEtNDJlMTlmODhlZmU3IiwicmlkIjoiNzVhZTdjMWYtZGExNS00YmJhLWFiYzktNTgyMTk0NWZhMTNjIn0.GZx7zOmsGfAc0Zdor2-d2iCXpY3lWXJTA-C8a1hZ-fJdB1t0DOtSyR4nSUJBC0sEpBF5gbChbQ9nW9W7fgfgDA"
	
	connStr := dbURL + "?authToken=" + token
	
	db, err := sql.Open("libsql", connStr)
	if err != nil {
		log.Fatalf("Failed to open: %v", err)
	}
	defer db.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping: %v", err)
	}
	fmt.Println("Ping successful!")
	
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM contacts").Scan(&count)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	fmt.Printf("Contact count: %d\n", count)
}
