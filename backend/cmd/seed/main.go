package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/sfid"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Connect to the database
	db, err := sql.Open("sqlite3", "../fastcrm.db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Generate IDs
	orgID := sfid.NewOrg()
	userID := sfid.NewUser()
	membershipID := sfid.NewMembership()

	log.Printf("Creating Green Valley Farms with org_id: %s", orgID)

	// 1. Create the farming company organization
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
		INSERT INTO organizations (id, name, slug, plan, is_active, settings, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, orgID, "Green Valley Farms", "green-valley-farms", "pro", 1, "{}", now, now)
	if err != nil {
		log.Fatal("Failed to create organization:", err)
	}
	log.Println("Created organization: Green Valley Farms")

	// 2. Create an admin user for this org
	// Password hash for "farm2024!" using bcrypt
	passwordHash := "$2a$10$rQtX.eSQ7vE3ZeJ.6Z.K4ORE8Ux2EV7h3r8bY6fXcQzQ8H.DhG7Je"
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, is_active, is_platform_admin, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, "admin@greenvalleyfarms.com", passwordHash, "Sarah", "Johnson", 1, 0, now, now)
	if err != nil {
		log.Fatal("Failed to create user:", err)
	}
	log.Println("Created user: admin@greenvalleyfarms.com (password: farm2024!)")

	// 3. Create membership linking user to org
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_org_memberships (id, user_id, org_id, role, is_default, joined_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, membershipID, userID, orgID, "owner", 1, now)
	if err != nil {
		log.Fatal("Failed to create membership:", err)
	}
	log.Println("Created membership for user in Green Valley Farms")

	// 4. Create custom entities for this org
	createCustomEntities(ctx, db, orgID)

	// 5. Create navigation tabs for this org
	createNavigationTabs(ctx, db, orgID)

	// 6. Populate with test data
	populateTestData(ctx, db, orgID, userID)

	log.Println("Seed completed successfully!")
	log.Printf("\n=== Login Credentials ===")
	log.Printf("Email: admin@greenvalleyfarms.com")
	log.Printf("Password: farm2024!")
	log.Printf("=========================\n")
}

func createCustomEntities(ctx context.Context, db *sql.DB, orgID string) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Create FarmCustomer entity
	farmCustomerID := sfid.NewEntity()
	_, err := db.ExecContext(ctx, `
		INSERT INTO entity_defs (id, org_id, name, label, label_plural, is_custom, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, farmCustomerID, orgID, "FarmCustomer", "Farm Customer", "Farm Customers", 1, now, now)
	if err != nil {
		log.Fatal("Failed to create FarmCustomer entity:", err)
	}
	log.Println("Created entity: FarmCustomer")

	// Create FarmCustomer fields
	createField(ctx, db, orgID, "FarmCustomer", "name", "text", "Name", true, 1)
	createField(ctx, db, orgID, "FarmCustomer", "email", "text", "Email", false, 2)
	createField(ctx, db, orgID, "FarmCustomer", "phone", "text", "Phone", false, 3)
	createField(ctx, db, orgID, "FarmCustomer", "address", "text", "Address", false, 4)
	createEnumField(ctx, db, orgID, "FarmCustomer", "customerType", "Customer Type", false, 5, []string{"Retail", "Wholesale", "Restaurant", "Farmers Market"})
	createEnumField(ctx, db, orgID, "FarmCustomer", "status", "Status", false, 6, []string{"Active", "Inactive", "Prospect"})

	// Create Crop entity
	cropID := sfid.NewEntity()
	_, err = db.ExecContext(ctx, `
		INSERT INTO entity_defs (id, org_id, name, label, label_plural, is_custom, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, cropID, orgID, "Crop", "Crop", "Crops", 1, now, now)
	if err != nil {
		log.Fatal("Failed to create Crop entity:", err)
	}
	log.Println("Created entity: Crop")

	// Create Crop fields
	createField(ctx, db, orgID, "Crop", "name", "text", "Name", true, 1)
	createEnumField(ctx, db, orgID, "Crop", "category", "Category", false, 2, []string{"Vegetables", "Fruits", "Grains", "Herbs", "Flowers"})
	createField(ctx, db, orgID, "Crop", "pricePerUnit", "currency", "Price Per Unit", false, 3)
	createEnumField(ctx, db, orgID, "Crop", "unit", "Unit", false, 4, []string{"lb", "kg", "bunch", "dozen", "bushel"})
	createField(ctx, db, orgID, "Crop", "inStock", "int", "In Stock", false, 5)
	createEnumField(ctx, db, orgID, "Crop", "season", "Season", false, 6, []string{"Spring", "Summer", "Fall", "Winter", "Year-round"})

	// Create FarmOrder entity
	farmOrderID := sfid.NewEntity()
	_, err = db.ExecContext(ctx, `
		INSERT INTO entity_defs (id, org_id, name, label, label_plural, is_custom, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, farmOrderID, orgID, "FarmOrder", "Farm Order", "Farm Orders", 1, now, now)
	if err != nil {
		log.Fatal("Failed to create FarmOrder entity:", err)
	}
	log.Println("Created entity: FarmOrder")

	// Create FarmOrder fields
	createField(ctx, db, orgID, "FarmOrder", "name", "text", "Order Number", true, 1)
	createLinkField(ctx, db, orgID, "FarmOrder", "customer", "Customer", false, 2, "FarmCustomer")
	createField(ctx, db, orgID, "FarmOrder", "orderDate", "date", "Order Date", false, 3)
	createField(ctx, db, orgID, "FarmOrder", "deliveryDate", "date", "Delivery Date", false, 4)
	createEnumField(ctx, db, orgID, "FarmOrder", "status", "Status", false, 5, []string{"Pending", "Confirmed", "In Progress", "Ready", "Delivered", "Cancelled"})
	createField(ctx, db, orgID, "FarmOrder", "totalAmount", "currency", "Total Amount", false, 6)
	createField(ctx, db, orgID, "FarmOrder", "notes", "text", "Notes", false, 7)

	// Create OrderLineItem entity
	lineItemID := sfid.NewEntity()
	_, err = db.ExecContext(ctx, `
		INSERT INTO entity_defs (id, org_id, name, label, label_plural, is_custom, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, lineItemID, orgID, "OrderLineItem", "Order Line Item", "Order Line Items", 1, now, now)
	if err != nil {
		log.Fatal("Failed to create OrderLineItem entity:", err)
	}
	log.Println("Created entity: OrderLineItem")

	// Create OrderLineItem fields
	createField(ctx, db, orgID, "OrderLineItem", "name", "text", "Name", true, 1)
	createLinkField(ctx, db, orgID, "OrderLineItem", "order", "Order", false, 2, "FarmOrder")
	createLinkField(ctx, db, orgID, "OrderLineItem", "crop", "Crop", false, 3, "Crop")
	createField(ctx, db, orgID, "OrderLineItem", "quantity", "int", "Quantity", false, 4)
	createField(ctx, db, orgID, "OrderLineItem", "unitPrice", "currency", "Unit Price", false, 5)
	createField(ctx, db, orgID, "OrderLineItem", "lineTotal", "currency", "Line Total", false, 6)

	// Create the tables for custom entities
	createCustomTable(ctx, db, "farmcustomers")
	createCustomTable(ctx, db, "crops")
	createCustomTable(ctx, db, "farmorders")
	createCustomTable(ctx, db, "orderlineitems")
}

func createField(ctx context.Context, db *sql.DB, orgID, entityName, fieldName, fieldType, label string, required bool, order int) {
	now := time.Now().UTC().Format(time.RFC3339)
	fieldID := sfid.NewFieldDef()

	isRequired := 0
	if required {
		isRequired = 1
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO field_defs (id, org_id, entity_name, name, type, label, is_required, is_custom, sort_order, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fieldID, orgID, entityName, fieldName, fieldType, label, isRequired, 1, order, now, now)
	if err != nil {
		log.Printf("Warning: Failed to create field %s.%s: %v", entityName, fieldName, err)
	}
}

func createEnumField(ctx context.Context, db *sql.DB, orgID, entityName, fieldName, label string, required bool, order int, options []string) {
	now := time.Now().UTC().Format(time.RFC3339)
	fieldID := sfid.NewFieldDef()

	optionsJSON, _ := json.Marshal(options)

	isRequired := 0
	if required {
		isRequired = 1
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO field_defs (id, org_id, entity_name, name, type, label, is_required, is_custom, sort_order, options, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fieldID, orgID, entityName, fieldName, "enum", label, isRequired, 1, order, string(optionsJSON), now, now)
	if err != nil {
		log.Printf("Warning: Failed to create enum field %s.%s: %v", entityName, fieldName, err)
	}
}

func createLinkField(ctx context.Context, db *sql.DB, orgID, entityName, fieldName, label string, required bool, order int, linkEntity string) {
	now := time.Now().UTC().Format(time.RFC3339)
	fieldID := sfid.NewFieldDef()

	isRequired := 0
	if required {
		isRequired = 1
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO field_defs (id, org_id, entity_name, name, type, label, is_required, is_custom, sort_order, link_entity, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fieldID, orgID, entityName, fieldName, "link", label, isRequired, 1, order, linkEntity, now, now)
	if err != nil {
		log.Printf("Warning: Failed to create link field %s.%s: %v", entityName, fieldName, err)
	}
}

func createCustomTable(ctx context.Context, db *sql.DB, tableName string) {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			name TEXT,
			data TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			modified_at TEXT DEFAULT CURRENT_TIMESTAMP
		)
	`, tableName)

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		log.Printf("Warning: Failed to create table %s: %v", tableName, err)
	} else {
		log.Printf("Created table: %s", tableName)
	}
}

func createNavigationTabs(ctx context.Context, db *sql.DB, orgID string) {
	now := time.Now().UTC().Format(time.RFC3339)

	tabs := []struct {
		label      string
		href       string
		entityName string
		order      int
	}{
		{"Home", "/", "", 0},
		{"Customers", "/FarmCustomer", "FarmCustomer", 1},
		{"Crops", "/Crop", "Crop", 2},
		{"Orders", "/FarmOrder", "FarmOrder", 3},
		{"Line Items", "/OrderLineItem", "OrderLineItem", 4},
	}

	for _, tab := range tabs {
		id := sfid.New("0Nt")
		_, err := db.ExecContext(ctx, `
			INSERT INTO navigation_tabs (id, org_id, label, href, icon, entity_name, sort_order, is_visible, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, orgID, tab.label, tab.href, "", tab.entityName, tab.order, 1, now, now)
		if err != nil {
			log.Printf("Warning: Failed to create navigation tab %s: %v", tab.label, err)
		}
	}
	log.Println("Created navigation tabs")
}

func populateTestData(ctx context.Context, db *sql.DB, orgID, userID string) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Create sample farm customers
	customers := []struct {
		id           string
		name         string
		email        string
		phone        string
		customerType string
		status       string
	}{
		{sfid.New("0FC"), "Fresh Eats Restaurant", "orders@fresheats.com", "555-0101", "Restaurant", "Active"},
		{sfid.New("0FC"), "Downtown Farmers Market", "vendor@dfmarket.com", "555-0102", "Farmers Market", "Active"},
		{sfid.New("0FC"), "Organic Grocers Co-op", "purchasing@organicgrocers.com", "555-0103", "Wholesale", "Active"},
		{sfid.New("0FC"), "The Healthy Kitchen", "chef@healthykitchen.com", "555-0104", "Restaurant", "Active"},
		{sfid.New("0FC"), "Green Basket Delivery", "info@greenbasket.com", "555-0105", "Retail", "Active"},
		{sfid.New("0FC"), "Farm to Table Catering", "events@f2tcatering.com", "555-0106", "Restaurant", "Prospect"},
		{sfid.New("0FC"), "Sunrise Bakery", "orders@sunrisebakery.com", "555-0107", "Wholesale", "Active"},
		{sfid.New("0FC"), "Local Harvest Store", "buyer@localharvest.com", "555-0108", "Retail", "Inactive"},
	}

	customerIDs := make(map[string]string)
	for _, c := range customers {
		data := map[string]interface{}{
			"email":        c.email,
			"phone":        c.phone,
			"customerType": c.customerType,
			"status":       c.status,
		}
		dataJSON, _ := json.Marshal(data)

		_, err := db.ExecContext(ctx, `
			INSERT INTO farmcustomers (id, name, data, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?)
		`, c.id, c.name, string(dataJSON), now, now)
		if err != nil {
			log.Printf("Warning: Failed to create customer %s: %v", c.name, err)
		}
		customerIDs[c.name] = c.id
	}
	log.Println("Created 8 farm customers")

	// Create sample crops
	crops := []struct {
		id           string
		name         string
		category     string
		pricePerUnit float64
		unit         string
		inStock      int
		season       string
	}{
		{sfid.New("0Cr"), "Heirloom Tomatoes", "Vegetables", 4.50, "lb", 150, "Summer"},
		{sfid.New("0Cr"), "Sweet Corn", "Vegetables", 0.75, "dozen", 200, "Summer"},
		{sfid.New("0Cr"), "Baby Spinach", "Vegetables", 6.00, "lb", 80, "Spring"},
		{sfid.New("0Cr"), "Butternut Squash", "Vegetables", 2.50, "lb", 120, "Fall"},
		{sfid.New("0Cr"), "Fresh Basil", "Herbs", 3.00, "bunch", 50, "Summer"},
		{sfid.New("0Cr"), "Strawberries", "Fruits", 5.00, "lb", 100, "Spring"},
		{sfid.New("0Cr"), "Blueberries", "Fruits", 7.00, "lb", 75, "Summer"},
		{sfid.New("0Cr"), "Kale", "Vegetables", 4.00, "bunch", 90, "Year-round"},
		{sfid.New("0Cr"), "Carrots", "Vegetables", 3.00, "lb", 200, "Year-round"},
		{sfid.New("0Cr"), "Sunflowers", "Flowers", 8.00, "bunch", 40, "Summer"},
		{sfid.New("0Cr"), "Lavender", "Herbs", 5.00, "bunch", 60, "Summer"},
		{sfid.New("0Cr"), "Winter Wheat", "Grains", 0.50, "bushel", 500, "Winter"},
	}

	cropIDs := make(map[string]string)
	for _, c := range crops {
		data := map[string]interface{}{
			"category":     c.category,
			"pricePerUnit": c.pricePerUnit,
			"unit":         c.unit,
			"inStock":      c.inStock,
			"season":       c.season,
		}
		dataJSON, _ := json.Marshal(data)

		_, err := db.ExecContext(ctx, `
			INSERT INTO crops (id, name, data, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?)
		`, c.id, c.name, string(dataJSON), now, now)
		if err != nil {
			log.Printf("Warning: Failed to create crop %s: %v", c.name, err)
		}
		cropIDs[c.name] = c.id
	}
	log.Println("Created 12 crops")

	// Create sample orders
	orders := []struct {
		id           string
		name         string
		customerName string
		status       string
		totalAmount  float64
		daysAgo      int
	}{
		{sfid.New("0FO"), "ORD-2024-001", "Fresh Eats Restaurant", "Delivered", 450.00, 7},
		{sfid.New("0FO"), "ORD-2024-002", "Downtown Farmers Market", "Delivered", 325.50, 5},
		{sfid.New("0FO"), "ORD-2024-003", "Organic Grocers Co-op", "Ready", 780.00, 2},
		{sfid.New("0FO"), "ORD-2024-004", "The Healthy Kitchen", "In Progress", 220.00, 1},
		{sfid.New("0FO"), "ORD-2024-005", "Green Basket Delivery", "Confirmed", 165.00, 0},
		{sfid.New("0FO"), "ORD-2024-006", "Sunrise Bakery", "Pending", 95.00, 0},
	}

	orderIDs := make(map[string]string)
	for _, o := range orders {
		orderDate := time.Now().AddDate(0, 0, -o.daysAgo).Format("2006-01-02")
		deliveryDate := time.Now().AddDate(0, 0, -o.daysAgo+2).Format("2006-01-02")

		data := map[string]interface{}{
			"customerId":   customerIDs[o.customerName],
			"orderDate":    orderDate,
			"deliveryDate": deliveryDate,
			"status":       o.status,
			"totalAmount":  o.totalAmount,
		}
		dataJSON, _ := json.Marshal(data)

		_, err := db.ExecContext(ctx, `
			INSERT INTO farmorders (id, name, data, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?)
		`, o.id, o.name, string(dataJSON), now, now)
		if err != nil {
			log.Printf("Warning: Failed to create order %s: %v", o.name, err)
		}
		orderIDs[o.name] = o.id
	}
	log.Println("Created 6 orders")

	// Create sample line items
	lineItems := []struct {
		orderName string
		cropName  string
		quantity  int
		unitPrice float64
	}{
		{"ORD-2024-001", "Heirloom Tomatoes", 50, 4.50},
		{"ORD-2024-001", "Fresh Basil", 30, 3.00},
		{"ORD-2024-001", "Baby Spinach", 20, 6.00},
		{"ORD-2024-002", "Sweet Corn", 100, 0.75},
		{"ORD-2024-002", "Strawberries", 40, 5.00},
		{"ORD-2024-002", "Sunflowers", 10, 8.00},
		{"ORD-2024-003", "Kale", 50, 4.00},
		{"ORD-2024-003", "Carrots", 100, 3.00},
		{"ORD-2024-003", "Blueberries", 40, 7.00},
		{"ORD-2024-004", "Heirloom Tomatoes", 30, 4.50},
		{"ORD-2024-004", "Lavender", 17, 5.00},
		{"ORD-2024-005", "Baby Spinach", 15, 6.00},
		{"ORD-2024-005", "Carrots", 25, 3.00},
		{"ORD-2024-006", "Fresh Basil", 15, 3.00},
		{"ORD-2024-006", "Strawberries", 10, 5.00},
	}

	for _, li := range lineItems {
		id := sfid.New("0LI")
		name := fmt.Sprintf("%s - %s", li.orderName, li.cropName)
		lineTotal := float64(li.quantity) * li.unitPrice

		data := map[string]interface{}{
			"orderId":   orderIDs[li.orderName],
			"cropId":    cropIDs[li.cropName],
			"quantity":  li.quantity,
			"unitPrice": li.unitPrice,
			"lineTotal": lineTotal,
		}
		dataJSON, _ := json.Marshal(data)

		_, err := db.ExecContext(ctx, `
			INSERT INTO orderlineitems (id, name, data, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?)
		`, id, name, string(dataJSON), now, now)
		if err != nil {
			log.Printf("Warning: Failed to create line item %s: %v", name, err)
		}
	}
	log.Println("Created 15 order line items")
}
