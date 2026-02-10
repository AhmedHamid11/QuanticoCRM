package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// Sample data for realistic generation
var firstNames = []string{
	"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda",
	"William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Charles", "Karen", "Christopher", "Nancy", "Daniel", "Lisa",
	"Matthew", "Betty", "Anthony", "Margaret", "Mark", "Sandra", "Donald", "Ashley",
	"Steven", "Kimberly", "Paul", "Emily", "Andrew", "Donna", "Joshua", "Michelle",
	"Kenneth", "Dorothy", "Kevin", "Carol", "Brian", "Amanda", "George", "Melissa",
	"Edward", "Deborah", "Ronald", "Stephanie", "Timothy", "Rebecca", "Jason", "Sharon",
	"Jeffrey", "Laura", "Ryan", "Cynthia", "Jacob", "Kathleen", "Gary", "Amy",
	"Nicholas", "Angela", "Eric", "Shirley", "Jonathan", "Anna", "Stephen", "Brenda",
	"Larry", "Pamela", "Justin", "Emma", "Scott", "Nicole", "Brandon", "Helen",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas",
	"Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White",
	"Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker", "Young",
	"Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores",
	"Green", "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
	"Carter", "Roberts", "Gomez", "Phillips", "Evans", "Turner", "Diaz", "Parker",
	"Cruz", "Edwards", "Collins", "Reyes", "Stewart", "Morris", "Morales", "Murphy",
}

var companyPrefixes = []string{
	"Global", "National", "American", "United", "First", "Pacific", "Atlantic", "Premier",
	"Advanced", "Superior", "Elite", "Prime", "Apex", "Pinnacle", "Summit", "Horizon",
	"Innovative", "Dynamic", "Strategic", "Integrated", "Unified", "Allied", "Consolidated", "Universal",
}

var companySuffixes = []string{
	"Industries", "Solutions", "Technologies", "Systems", "Services", "Group", "Corp", "Inc",
	"Enterprises", "Holdings", "Partners", "Associates", "Consulting", "Dynamics", "Logistics", "Networks",
	"Ventures", "Capital", "Resources", "Management", "Development", "International", "Worldwide", "Global",
}

var industries = []string{
	"Technology", "Healthcare", "Finance", "Manufacturing", "Retail", "Education",
	"Real Estate", "Construction", "Transportation", "Energy", "Telecommunications",
	"Media", "Hospitality", "Agriculture", "Automotive", "Aerospace",
}

var accountTypes = []string{
	"Customer", "Partner", "Prospect", "Vendor", "Investor", "Other",
}

var cities = []string{
	"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia",
	"San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville",
	"Fort Worth", "Columbus", "Charlotte", "San Francisco", "Indianapolis", "Seattle",
	"Denver", "Boston", "Nashville", "Baltimore", "Oklahoma City", "Portland",
	"Las Vegas", "Memphis", "Louisville", "Milwaukee", "Albuquerque", "Tucson",
}

var states = []string{
	"NY", "CA", "IL", "TX", "AZ", "PA", "FL", "OH", "NC", "WA",
	"CO", "MA", "TN", "MD", "OK", "OR", "NV", "KY", "WI", "NM",
}

var streets = []string{
	"Main St", "Oak Ave", "Maple Dr", "Cedar Ln", "Pine St", "Elm St",
	"Washington Blvd", "Park Ave", "Lake Dr", "Hill Rd", "River Rd", "Forest Ave",
	"Commerce Dr", "Industrial Pkwy", "Technology Way", "Innovation Blvd",
}

var salutations = []string{"Mr.", "Ms.", "Mrs.", "Dr.", ""}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Get org_id from command line or use default
	orgID := "00Dc18f5b2b0a4b2b9" // Quantico Operations
	if len(os.Args) > 1 {
		orgID = os.Args[1]
	}

	numAccounts := 2000
	numContacts := 8000

	fmt.Println("-- Load test data for Quantico CRM")
	fmt.Println("-- Generated:", time.Now().Format(time.RFC3339))
	fmt.Printf("-- Org ID: %s\n", orgID)
	fmt.Printf("-- Accounts: %d, Contacts: %d\n\n", numAccounts, numContacts)

	// Generate accounts
	accountIDs := make([]string, numAccounts)
	fmt.Println("-- ============ ACCOUNTS ============")
	fmt.Println("BEGIN TRANSACTION;")

	for i := 0; i < numAccounts; i++ {
		id := generateID("00A")
		accountIDs[i] = id

		name := generateCompanyName()
		website := fmt.Sprintf("https://www.%s.com", strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, " ", ""), ",", "")))
		email := fmt.Sprintf("info@%s.com", strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, " ", ""), ",", "")))
		phone := generatePhone()
		accType := accountTypes[rand.Intn(len(accountTypes))]
		industry := industries[rand.Intn(len(industries))]
		city := cities[rand.Intn(len(cities))]
		state := states[rand.Intn(len(states))]
		street := fmt.Sprintf("%d %s", rand.Intn(9999)+1, streets[rand.Intn(len(streets))])
		postal := fmt.Sprintf("%05d", rand.Intn(99999))
		createdAt := randomDate()

		fmt.Printf(`INSERT INTO accounts (id, org_id, name, website, email_address, phone_number, type, industry, billing_address_street, billing_address_city, billing_address_state, billing_address_country, billing_address_postal_code, created_at, modified_at, deleted) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', 'USA', '%s', '%s', '%s', 0);`+"\n",
			id, orgID, escapeSql(name), website, email, phone, accType, industry, street, city, state, postal, createdAt, createdAt)
	}

	fmt.Println("COMMIT;")
	fmt.Println()

	// Generate contacts
	fmt.Println("-- ============ CONTACTS ============")
	fmt.Println("BEGIN TRANSACTION;")

	for i := 0; i < numContacts; i++ {
		id := generateID("00C")
		firstName := firstNames[rand.Intn(len(firstNames))]
		lastName := lastNames[rand.Intn(len(lastNames))]
		salutation := salutations[rand.Intn(len(salutations))]
		email := fmt.Sprintf("%s.%s%d@email.com", strings.ToLower(firstName), strings.ToLower(lastName), rand.Intn(999))
		phone := generatePhone()
		city := cities[rand.Intn(len(cities))]
		state := states[rand.Intn(len(states))]
		street := fmt.Sprintf("%d %s", rand.Intn(9999)+1, streets[rand.Intn(len(streets))])
		postal := fmt.Sprintf("%05d", rand.Intn(99999))
		accountID := accountIDs[rand.Intn(len(accountIDs))]
		createdAt := randomDate()

		fmt.Printf(`INSERT INTO contacts (id, org_id, salutation_name, first_name, last_name, email_address, phone_number, address_street, address_city, address_state, address_country, address_postal_code, account_id, created_at, modified_at, deleted) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', 'USA', '%s', '%s', '%s', '%s', 0);`+"\n",
			id, orgID, salutation, firstName, lastName, email, phone, street, city, state, postal, accountID, createdAt, createdAt)
	}

	fmt.Println("COMMIT;")
	fmt.Println()
	fmt.Println("-- Done! Total records:", numAccounts+numContacts)
}

func generateID(prefix string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 15)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return prefix + string(result)
}

func generateCompanyName() string {
	prefix := companyPrefixes[rand.Intn(len(companyPrefixes))]
	suffix := companySuffixes[rand.Intn(len(companySuffixes))]
	return fmt.Sprintf("%s %s", prefix, suffix)
}

func generatePhone() string {
	return fmt.Sprintf("(%03d) %03d-%04d", rand.Intn(900)+100, rand.Intn(900)+100, rand.Intn(10000))
}

func randomDate() string {
	// Random date in the last 2 years
	days := rand.Intn(730)
	t := time.Now().AddDate(0, 0, -days)
	return t.Format("2006-01-02T15:04:05Z")
}

func escapeSql(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
