package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/sfid"
)

// ProvisionCREBroker creates the complete CRE (Commercial Real Estate) brokerage CRM setup.
// This includes entities, fields, layouts, navigation, bearings, and related lists for:
// - Account (Landlord/Tenant/Broker types — uses existing standard entity)
// - Contact (uses existing standard entity)
// - Lead (space-seeking tenant leads)
// - Property (commercial properties for lease)
// - Deal (lease transactions with commission tracking)
// - Task (uses existing standard entity)
func (s *ProvisioningService) ProvisionCREBroker(ctx context.Context, orgID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Starting CRE Broker provisioning for org %s", orgID)

	// ========================================================================
	// ENTITIES — only create new ones (Account, Contact, Task already exist)
	// ========================================================================
	newEntities := []struct{ name, plural string }{
		{"Lead", "Leads"},
		{"Property", "Properties"},
		{"Deal", "Deals"},
	}
	for _, e := range newEntities {
		if err := s.createCREEntity(ctx, orgID, e.name, e.plural, now); err != nil {
			log.Printf("[Provisioning] Warning: failed to create %s entity: %v", e.name, err)
		} else {
			log.Printf("[Provisioning] Created entity %s", e.name)
		}
	}

	// ========================================================================
	// UPDATE ACCOUNT TYPE FIELD — set CRE-specific options
	// ========================================================================
	_, err := s.db.ExecContext(ctx, `
		UPDATE field_defs
		SET options = '["Landlord","Tenant","Broker"]', modified_at = ?
		WHERE org_id = ? AND entity_name = 'Account' AND name = 'type'
	`, now, orgID)
	if err != nil {
		log.Printf("[Provisioning] Warning: failed to update Account type options: %v", err)
	} else {
		log.Printf("[Provisioning] Updated Account type field options for CRE")
	}

	// ========================================================================
	// LEAD FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Lead", "companyName", "Company Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Lead", "firstName", "First Name", "varchar", true, 2, now)
	s.createField(ctx, orgID, "Lead", "lastName", "Last Name", "varchar", true, 3, now)
	s.createField(ctx, orgID, "Lead", "emailAddress", "Email", "email", false, 4, now)
	s.createField(ctx, orgID, "Lead", "phoneNumber", "Phone", "phone", false, 5, now)
	s.createEnumField(ctx, orgID, "Lead", "status", "Status", []string{"New", "Contacted", "Qualified", "Unqualified", "Converted"}, 6, now)
	s.createEnumField(ctx, orgID, "Lead", "spaceTypeNeeded", "Space Type Needed", []string{"Office", "Retail", "Industrial", "Mixed-Use"}, 10, now)
	s.createField(ctx, orgID, "Lead", "estimatedSqFt", "Estimated Sq Ft", "int", false, 11, now)
	s.createField(ctx, orgID, "Lead", "estimatedBudget", "Estimated Budget", "currency", false, 12, now)
	s.createField(ctx, orgID, "Lead", "budgetPerSqFt", "Budget Per Sq Ft", "currency", false, 13, now)
	s.createField(ctx, orgID, "Lead", "moveInTimeline", "Move-In Timeline", "varchar", false, 14, now)
	s.createEnumField(ctx, orgID, "Lead", "source", "Source", []string{"Website", "Referral", "Cold Call", "Networking Event", "CoStar", "LoopNet", "Sign Call"}, 15, now)
	s.createLinkField(ctx, orgID, "Lead", "assignedUserId", "Assigned To", "User", 20, now)
	s.createField(ctx, orgID, "Lead", "description", "Description", "text", false, 30, now)
	s.createField(ctx, orgID, "Lead", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Lead", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// PROPERTY FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Property", "name", "Property Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Property", "addressStreet", "Street", "varchar", false, 2, now)
	s.createField(ctx, orgID, "Property", "addressCity", "City", "varchar", false, 3, now)
	s.createField(ctx, orgID, "Property", "addressState", "State", "varchar", false, 4, now)
	s.createField(ctx, orgID, "Property", "addressPostalCode", "Postal Code", "varchar", false, 5, now)
	s.createLinkField(ctx, orgID, "Property", "landlordId", "Landlord", "Account", 10, now)
	s.createLinkField(ctx, orgID, "Property", "primaryContactId", "Primary Contact", "Contact", 11, now)
	s.createField(ctx, orgID, "Property", "totalSqFt", "Total Sq Ft", "int", false, 20, now)
	s.createField(ctx, orgID, "Property", "availableSqFt", "Available Sq Ft", "int", false, 21, now)
	s.createEnumField(ctx, orgID, "Property", "status", "Status", []string{"Available", "Leased", "Pending", "Off Market"}, 22, now)
	s.createField(ctx, orgID, "Property", "askingPricePerSqFt", "Asking Price/Sq Ft", "currency", false, 23, now)
	s.createField(ctx, orgID, "Property", "availabilityDate", "Availability Date", "date", false, 24, now)
	s.createEnumField(ctx, orgID, "Property", "propertyType", "Property Type", []string{"Office", "Retail", "Industrial", "Mixed-Use"}, 25, now)
	s.createField(ctx, orgID, "Property", "description", "Description", "text", false, 30, now)
	s.createField(ctx, orgID, "Property", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Property", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// DEAL FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Deal", "name", "Deal Name", "varchar", true, 1, now)
	s.createLinkField(ctx, orgID, "Deal", "propertyId", "Property", "Property", 2, now)
	s.createLinkField(ctx, orgID, "Deal", "tenantId", "Tenant", "Account", 3, now)
	s.createLinkField(ctx, orgID, "Deal", "tenantContactId", "Tenant Contact", "Contact", 4, now)
	s.createLinkField(ctx, orgID, "Deal", "landlordId", "Landlord", "Account", 5, now)
	s.createLinkField(ctx, orgID, "Deal", "landlordContactId", "Landlord Contact", "Contact", 6, now)
	s.createField(ctx, orgID, "Deal", "leasingBroker", "Leasing Broker", "varchar", false, 7, now)
	s.createEnumField(ctx, orgID, "Deal", "represents", "Represents", []string{"Landlord", "Tenant", "Both"}, 8, now)
	s.createField(ctx, orgID, "Deal", "dealValue", "Deal Value", "currency", false, 10, now)
	s.createField(ctx, orgID, "Deal", "sqFootageLeased", "Sq Footage Leased", "int", false, 11, now)
	s.createField(ctx, orgID, "Deal", "leaseTermLength", "Lease Term Length", "varchar", false, 12, now)
	s.createField(ctx, orgID, "Deal", "baseRent", "Base Rent", "currency", false, 13, now)
	s.createField(ctx, orgID, "Deal", "commissionPctLandlord", "Commission % Landlord", "float", false, 20, now)
	s.createField(ctx, orgID, "Deal", "commissionAmtLandlord", "Commission $ Landlord", "currency", false, 21, now)
	s.createField(ctx, orgID, "Deal", "commissionPctTenant", "Commission % Tenant", "float", false, 22, now)
	s.createField(ctx, orgID, "Deal", "commissionAmtTenant", "Commission $ Tenant", "currency", false, 23, now)
	s.createField(ctx, orgID, "Deal", "totalCommission", "Total Commission", "currency", false, 24, now)
	s.createEnumField(ctx, orgID, "Deal", "status", "Status", []string{
		"Pipeline", "Under Negotiation", "Pending", "Offer Submitted", "Offer Accepted", "Closed Won", "Closed Lost",
	}, 30, now)
	s.createField(ctx, orgID, "Deal", "closeDate", "Close Date", "date", false, 31, now)
	s.createField(ctx, orgID, "Deal", "expectedCloseDate", "Expected Close Date", "date", false, 32, now)
	s.createEnumField(ctx, orgID, "Deal", "dealType", "Deal Type", []string{"New Lease", "Renewal", "Expansion", "Sublease"}, 33, now)
	s.createField(ctx, orgID, "Deal", "leaseStartDate", "Lease Start Date", "date", false, 40, now)
	s.createField(ctx, orgID, "Deal", "leaseEndDate", "Lease End Date", "date", false, 41, now)
	s.createEnumField(ctx, orgID, "Deal", "commissionStatus", "Commission Status", []string{"Not Yet Earned", "Earned", "Paid"}, 42, now)
	s.createField(ctx, orgID, "Deal", "commissionPaidDate", "Commission Paid Date", "date", false, 43, now)
	s.createField(ctx, orgID, "Deal", "notes", "Notes", "text", false, 50, now)
	s.createField(ctx, orgID, "Deal", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Deal", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// LIST LAYOUTS
	// ========================================================================
	s.createLayout(ctx, orgID, "Lead", "list",
		`["companyName","firstName","lastName","emailAddress","status","spaceTypeNeeded","estimatedSqFt","source","createdAt"]`, now)
	s.createLayout(ctx, orgID, "Property", "list",
		`["name","addressCity","addressState","landlordId","totalSqFt","availableSqFt","status","askingPricePerSqFt","propertyType"]`, now)
	s.createLayout(ctx, orgID, "Deal", "list",
		`["name","propertyId","tenantId","landlordId","dealValue","status","dealType","expectedCloseDate","totalCommission"]`, now)

	// ========================================================================
	// DETAIL LAYOUTS
	// ========================================================================

	// Lead detail layout
	s.createLayout(ctx, orgID, "Lead", "detail", `[
		{"label":"Overview","rows":[
			[{"field":"companyName"}],
			[{"field":"firstName"},{"field":"lastName"}],
			[{"field":"emailAddress"},{"field":"phoneNumber"}],
			[{"field":"status"},{"field":"source"}],
			[{"field":"assignedUserId"}]
		]},
		{"label":"Space Requirements","rows":[
			[{"field":"spaceTypeNeeded"},{"field":"estimatedSqFt"}],
			[{"field":"estimatedBudget"},{"field":"budgetPerSqFt"}],
			[{"field":"moveInTimeline"}]
		]},
		{"label":"Description","rows":[
			[{"field":"description"}]
		]}
	]`, now)

	// Property detail layout
	s.createLayout(ctx, orgID, "Property", "detail", `[
		{"label":"Overview","rows":[
			[{"field":"name"},{"field":"propertyType"}],
			[{"field":"landlordId"},{"field":"primaryContactId"}],
			[{"field":"status"}]
		]},
		{"label":"Address","rows":[
			[{"field":"addressStreet"}],
			[{"field":"addressCity"},{"field":"addressState"}],
			[{"field":"addressPostalCode"}]
		]},
		{"label":"Space Info","rows":[
			[{"field":"totalSqFt"},{"field":"availableSqFt"}],
			[{"field":"askingPricePerSqFt"},{"field":"availabilityDate"}]
		]},
		{"label":"Description","rows":[
			[{"field":"description"}]
		]}
	]`, now)

	// Deal detail layout
	s.createLayout(ctx, orgID, "Deal", "detail", `[
		{"label":"Deal Info","rows":[
			[{"field":"name"},{"field":"dealType"}],
			[{"field":"status"},{"field":"represents"}],
			[{"field":"propertyId"},{"field":"leasingBroker"}]
		]},
		{"label":"Parties","rows":[
			[{"field":"tenantId"},{"field":"tenantContactId"}],
			[{"field":"landlordId"},{"field":"landlordContactId"}]
		]},
		{"label":"Financials","rows":[
			[{"field":"dealValue"},{"field":"sqFootageLeased"}],
			[{"field":"baseRent"},{"field":"leaseTermLength"}]
		]},
		{"label":"Commission","rows":[
			[{"field":"commissionPctLandlord"},{"field":"commissionAmtLandlord"}],
			[{"field":"commissionPctTenant"},{"field":"commissionAmtTenant"}],
			[{"field":"totalCommission"}],
			[{"field":"commissionStatus"},{"field":"commissionPaidDate"}]
		]},
		{"label":"Dates","rows":[
			[{"field":"expectedCloseDate"},{"field":"closeDate"}],
			[{"field":"leaseStartDate"},{"field":"leaseEndDate"}]
		]},
		{"label":"Notes","rows":[
			[{"field":"notes"}]
		]}
	]`, now)

	// ========================================================================
	// NAVIGATION TABS — replace defaults with CRE-specific tabs
	// ========================================================================
	// Delete existing navigation tabs for this org first, then create CRE tabs
	_, err = s.db.ExecContext(ctx, `DELETE FROM navigation_tabs WHERE org_id = ?`, orgID)
	if err != nil {
		log.Printf("[Provisioning] Warning: failed to clear navigation tabs: %v", err)
	}

	s.createNavTab(ctx, orgID, "Home", "/", "", 0, true, now)
	s.createNavTab(ctx, orgID, "Accounts", "/accounts", "Account", 1, true, now)
	s.createNavTab(ctx, orgID, "Contacts", "/contacts", "Contact", 2, true, now)
	s.createNavTab(ctx, orgID, "Properties", "/properties", "Property", 3, true, now)
	s.createNavTab(ctx, orgID, "Leads", "/leads", "Lead", 4, true, now)
	s.createNavTab(ctx, orgID, "Deals", "/deals", "Deal", 5, true, now)
	s.createNavTab(ctx, orgID, "Tasks", "/tasks", "Task", 6, true, now)

	// ========================================================================
	// BEARINGS (Stage Progress Indicators)
	// ========================================================================
	s.createBearing(ctx, orgID, "Lead", "Lead Pipeline", "status", 1, now)
	s.createBearing(ctx, orgID, "Deal", "Deal Pipeline", "status", 1, now)
	s.createBearing(ctx, orgID, "Property", "Property Status", "status", 1, now)

	// ========================================================================
	// RELATED LIST CONFIGS
	// ========================================================================
	type relatedListDef struct {
		entityType     string
		relatedEntity  string
		lookupField    string
		label          string
		displayFields  string
		sortOrder      int
		defaultSort    string
		defaultSortDir string
		pageSize       int
	}

	relatedLists := []relatedListDef{
		// Account → Contacts (standard — accountId lookup)
		{
			entityType:     "Account",
			relatedEntity:  "Contact",
			lookupField:    "accountId",
			label:          "Contacts",
			displayFields:  `[{"field":"firstName","label":"First Name","position":1},{"field":"lastName","label":"Last Name","position":2},{"field":"emailAddress","label":"Email","position":3},{"field":"phoneNumber","label":"Phone","position":4}]`,
			sortOrder:      1,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
		// Account → Properties (landlordId lookup)
		{
			entityType:     "Account",
			relatedEntity:  "Property",
			lookupField:    "landlordId",
			label:          "Properties",
			displayFields:  `[{"field":"name","label":"Name","position":1},{"field":"addressCity","label":"City","position":2},{"field":"status","label":"Status","position":3},{"field":"propertyType","label":"Type","position":4}]`,
			sortOrder:      2,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
		// Account → Deals as Tenant (tenantId lookup)
		{
			entityType:     "Account",
			relatedEntity:  "Deal",
			lookupField:    "tenantId",
			label:          "Deals (Tenant)",
			displayFields:  `[{"field":"name","label":"Name","position":1},{"field":"propertyId","label":"Property","position":2},{"field":"dealValue","label":"Value","position":3},{"field":"status","label":"Status","position":4}]`,
			sortOrder:      3,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
		// Account → Deals as Landlord (landlordId lookup)
		{
			entityType:     "Account",
			relatedEntity:  "Deal",
			lookupField:    "landlordId",
			label:          "Deals (Landlord)",
			displayFields:  `[{"field":"name","label":"Name","position":1},{"field":"propertyId","label":"Property","position":2},{"field":"dealValue","label":"Value","position":3},{"field":"status","label":"Status","position":4}]`,
			sortOrder:      4,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
		// Property → Deals (propertyId lookup)
		{
			entityType:     "Property",
			relatedEntity:  "Deal",
			lookupField:    "propertyId",
			label:          "Deals",
			displayFields:  `[{"field":"name","label":"Name","position":1},{"field":"tenantId","label":"Tenant","position":2},{"field":"dealValue","label":"Value","position":3},{"field":"status","label":"Status","position":4}]`,
			sortOrder:      1,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
		// Contact → Deals as Tenant Contact (tenantContactId lookup)
		{
			entityType:     "Contact",
			relatedEntity:  "Deal",
			lookupField:    "tenantContactId",
			label:          "Deals (Tenant)",
			displayFields:  `[{"field":"name","label":"Name","position":1},{"field":"propertyId","label":"Property","position":2},{"field":"dealValue","label":"Value","position":3},{"field":"status","label":"Status","position":4}]`,
			sortOrder:      1,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
		// Contact → Deals as Landlord Contact (landlordContactId lookup)
		{
			entityType:     "Contact",
			relatedEntity:  "Deal",
			lookupField:    "landlordContactId",
			label:          "Deals (Landlord)",
			displayFields:  `[{"field":"name","label":"Name","position":1},{"field":"propertyId","label":"Property","position":2},{"field":"dealValue","label":"Value","position":3},{"field":"status","label":"Status","position":4}]`,
			sortOrder:      2,
			defaultSort:    "createdAt",
			defaultSortDir: "desc",
			pageSize:       5,
		},
	}

	for _, cfg := range relatedLists {
		id := sfid.New("0Rl")
		_, err := s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO related_list_configs
			(id, org_id, entity_type, related_entity, lookup_field, label, enabled,
			 display_fields, sort_order, default_sort, default_sort_dir, page_size,
			 created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?)
		`, id, orgID, cfg.entityType, cfg.relatedEntity, cfg.lookupField, cfg.label,
			cfg.displayFields, cfg.sortOrder, cfg.defaultSort, cfg.defaultSortDir, cfg.pageSize,
			now, now)
		if err != nil {
			log.Printf("[Provisioning] Warning: failed to create related list %s → %s (%s): %v",
				cfg.entityType, cfg.relatedEntity, cfg.lookupField, err)
		} else {
			log.Printf("[Provisioning] Created related list %s → %s (%s)",
				cfg.entityType, cfg.relatedEntity, cfg.lookupField)
		}
	}

	log.Printf("[Provisioning] Completed CRE Broker provisioning for org %s", orgID)
	return nil
}

// createCREEntity creates an entity with CRE-specific lookup config
func (s *ProvisioningService) createCREEntity(ctx context.Context, orgID, name, plural, now string) error {
	id := sfid.New("0Et")

	displayField, searchFields := getCREEntityLookupConfig(name)

	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_defs (id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, display_field, search_fields, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, '', '', 0, 1, 0, 0, ?, ?, ?, ?)
	`, id, orgID, name, name, plural, displayField, searchFields, now, now)
	return err
}

// getCREEntityLookupConfig returns display_field and search_fields for CRE entities
func getCREEntityLookupConfig(entityName string) (displayField string, searchFields string) {
	switch entityName {
	case "Lead":
		return "company_name", `["company_name", "first_name", "last_name", "email_address"]`
	case "Property":
		return "name", `["name", "address_city", "address_state"]`
	case "Deal":
		return "name", `["name"]`
	default:
		return "name", `["name"]`
	}
}

// ProvisionCREBrokerComplete runs full CRE provisioning including metadata, navigation, and sample data
func (s *ProvisioningService) ProvisionCREBrokerComplete(ctx context.Context, orgID string) error {
	if err := s.ProvisionCREBroker(ctx, orgID); err != nil {
		return fmt.Errorf("failed to provision CRE metadata: %w", err)
	}
	// Navigation is already created inside ProvisionCREBroker (custom tabs)
	s.ProvisionCREBrokerSampleData(ctx, orgID)
	return nil
}

// ProvisionCREBrokerSampleData seeds realistic CRE brokerage data:
// 10 accounts (Landlord/Tenant/Broker), 15 contacts, 6 properties, 8 leads, 8 deals
func (s *ProvisioningService) ProvisionCREBrokerSampleData(ctx context.Context, orgID string) {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Creating CRE Broker sample data for org %s", orgID)

	// ========================================================================
	// ACCOUNTS (10 total: 3 Landlord, 5 Tenant, 2 Broker)
	// ========================================================================
	type accountDef struct {
		name, accountType, industry, website, phone, email, billingCity, billingState string
	}

	accounts := []accountDef{
		// Landlords
		{"Brookfield Property Partners", "Landlord", "Real Estate", "brookfieldpropertypartners.com", "212-417-7000", "info@brookfield.com", "New York", "NY"},
		{"Mack-Cali Realty", "Landlord", "Real Estate", "mack-cali.com", "732-590-1000", "info@mack-cali.com", "Jersey City", "NJ"},
		{"Prologis Industrial", "Landlord", "Industrial Real Estate", "prologis.com", "415-394-9000", "info@prologis.com", "San Francisco", "CA"},
		// Tenants
		{"WeWork Enterprise", "Tenant", "Flexible Workspace", "wework.com", "646-389-3922", "enterprise@wework.com", "New York", "NY"},
		{"Kirkland & Ellis LLP", "Tenant", "Legal Services", "kirkland.com", "312-862-2000", "realestate@kirkland.com", "Chicago", "IL"},
		{"Amazon Last Mile", "Tenant", "Logistics", "amazon.com", "206-266-1000", "realestate@amazon.com", "Seattle", "WA"},
		{"Starbucks Regional", "Tenant", "Food & Beverage", "starbucks.com", "206-447-1575", "retail@starbucks.com", "Seattle", "WA"},
		{"Regus Flex", "Tenant", "Flexible Workspace", "regus.com", "203-539-5900", "leasing@regus.com", "Stamford", "CT"},
		// Brokers
		{"Marcus & Millichap", "Broker", "Commercial Real Estate Services", "marcusmillichap.com", "818-212-2250", "info@marcusmillichap.com", "Calabasas", "CA"},
		{"CBRE Advisory", "Broker", "Commercial Real Estate Services", "cbre.com", "212-984-8000", "advisory@cbre.com", "New York", "NY"},
	}

	accountIDs := make(map[string]string)
	for _, a := range accounts {
		id := sfid.NewAccount()
		accountIDs[a.name] = id
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO accounts (id, org_id, name, type, industry, website, phone_number, email_address,
				billing_address_city, billing_address_state, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, id, orgID, a.name, a.accountType, a.industry, a.website, a.phone, a.email,
			a.billingCity, a.billingState, now, now)
		if err != nil {
			log.Printf("[Provisioning] Warning: failed to create account %s: %v", a.name, err)
		}
	}
	log.Printf("[Provisioning] Created %d accounts", len(accounts))

	// ========================================================================
	// CONTACTS (15 total — 1-2 per account with CRE titles)
	// ========================================================================
	type contactDef struct {
		firstName, lastName, title, email, phone, accountName string
	}

	contacts := []contactDef{
		// Brookfield Property Partners
		{"Michael", "Emory", "VP of Leasing", "m.emory@brookfield.com", "212-417-7100", "Brookfield Property Partners"},
		{"Sarah", "Chen", "Director of Asset Management", "s.chen@brookfield.com", "212-417-7200", "Brookfield Property Partners"},
		// Mack-Cali Realty
		{"David", "Sarnoff", "SVP Leasing", "d.sarnoff@mack-cali.com", "732-590-1100", "Mack-Cali Realty"},
		{"Patricia", "Okoye", "Property Manager", "p.okoye@mack-cali.com", "732-590-1200", "Mack-Cali Realty"},
		// Prologis Industrial
		{"Robert", "Hamid", "Regional VP Leasing", "r.hamid@prologis.com", "415-394-9100", "Prologis Industrial"},
		// WeWork Enterprise
		{"Amanda", "Torres", "Director of Real Estate", "a.torres@wework.com", "646-389-3930", "WeWork Enterprise"},
		// Kirkland & Ellis LLP
		{"James", "Whitfield", "CFO / Head of Real Estate", "j.whitfield@kirkland.com", "312-862-2100", "Kirkland & Ellis LLP"},
		{"Nicole", "Park", "Office Operations Manager", "n.park@kirkland.com", "312-862-2200", "Kirkland & Ellis LLP"},
		// Amazon Last Mile
		{"Carlos", "Rivera", "Senior Real Estate Manager", "c.rivera@amazon.com", "206-266-1200", "Amazon Last Mile"},
		// Starbucks Regional
		{"Lisa", "Nguyen", "Regional Real Estate Director", "l.nguyen@starbucks.com", "206-447-1600", "Starbucks Regional"},
		// Regus Flex
		{"Thomas", "Blake", "Tenant Representative", "t.blake@regus.com", "203-539-5950", "Regus Flex"},
		// Marcus & Millichap
		{"Kevin", "Marks", "Senior Broker", "k.marks@marcusmillichap.com", "818-212-2300", "Marcus & Millichap"},
		{"Diana", "Wolfe", "Research Analyst", "d.wolfe@marcusmillichap.com", "818-212-2310", "Marcus & Millichap"},
		// CBRE Advisory
		{"Steven", "Dumont", "Managing Director", "s.dumont@cbre.com", "212-984-8100", "CBRE Advisory"},
		{"Jennifer", "Castillo", "Transaction Manager", "j.castillo@cbre.com", "212-984-8200", "CBRE Advisory"},
	}

	contactIDs := make(map[string]string)
	for _, c := range contacts {
		id := sfid.NewContact()
		contactKey := c.firstName + " " + c.lastName
		contactIDs[contactKey] = id
		accountID := accountIDs[c.accountName]

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO contacts (id, org_id, first_name, last_name, email_address, phone_number,
				account_id, account_name, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, id, orgID, c.firstName, c.lastName, c.email, c.phone,
			accountID, c.accountName, now, now)
		if err != nil {
			log.Printf("[Provisioning] Warning: failed to create contact %s %s: %v", c.firstName, c.lastName, err)
		}
	}
	log.Printf("[Provisioning] Created %d contacts", len(contacts))

	// ========================================================================
	// PROPERTIES (6 commercial properties)
	// ========================================================================
	type propertyDef struct {
		name, street, city, state, postalCode string
		landlordName, contactName             string
		totalSqFt, availableSqFt             int
		status, propertyType                  string
		askingPricePerSqFt                    float64
		availabilityDate                      string
	}

	properties := []propertyDef{
		{
			name: "One Liberty Plaza", street: "1 Liberty Plaza", city: "New York", state: "NY", postalCode: "10006",
			landlordName: "Brookfield Property Partners", contactName: "Michael Emory",
			totalSqFt: 2200000, availableSqFt: 325000,
			status: "Available", propertyType: "Office",
			askingPricePerSqFt: 72.50, availabilityDate: "2026-04-01",
		},
		{
			name: "Harborside Financial Center", street: "3 Second Street", city: "Jersey City", state: "NJ", postalCode: "07311",
			landlordName: "Mack-Cali Realty", contactName: "David Sarnoff",
			totalSqFt: 4300000, availableSqFt: 850000,
			status: "Pending", propertyType: "Office",
			askingPricePerSqFt: 45.00, availabilityDate: "2026-06-01",
		},
		{
			name: "Gateway Center", street: "100 Gateway Center Pkwy", city: "Newark", state: "NJ", postalCode: "07102",
			landlordName: "Mack-Cali Realty", contactName: "Patricia Okoye",
			totalSqFt: 1100000, availableSqFt: 220000,
			status: "Available", propertyType: "Office",
			askingPricePerSqFt: 38.00, availabilityDate: "2026-03-15",
		},
		{
			name: "Prologis Meadowlands", street: "300 Industrial Ave", city: "Carlstadt", state: "NJ", postalCode: "07072",
			landlordName: "Prologis Industrial", contactName: "Robert Hamid",
			totalSqFt: 800000, availableSqFt: 800000,
			status: "Available", propertyType: "Industrial",
			askingPricePerSqFt: 12.50, availabilityDate: "2026-04-15",
		},
		{
			name: "225 West Wacker Drive", street: "225 W Wacker Dr", city: "Chicago", state: "IL", postalCode: "60606",
			landlordName: "Brookfield Property Partners", contactName: "Sarah Chen",
			totalSqFt: 950000, availableSqFt: 0,
			status: "Leased", propertyType: "Office",
			askingPricePerSqFt: 55.00, availabilityDate: "",
		},
		{
			name: "Prologis O'Hare Logistics Center", street: "2400 Mannheim Rd", city: "Elk Grove Village", state: "IL", postalCode: "60007",
			landlordName: "Prologis Industrial", contactName: "Robert Hamid",
			totalSqFt: 500000, availableSqFt: 500000,
			status: "Available", propertyType: "Industrial",
			askingPricePerSqFt: 10.75, availabilityDate: "2026-05-01",
		},
	}

	propertyIDs := make(map[string]string)
	for _, p := range properties {
		id := sfid.New("0Pp")
		propertyIDs[p.name] = id
		landlordID := accountIDs[p.landlordName]
		contactID := contactIDs[p.contactName]

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO propertys (id, org_id, name, address_street, address_city, address_state, address_postal_code,
				landlord_id, landlord_id_name, primary_contact_id, primary_contact_id_name,
				total_sq_ft, available_sq_ft, status, asking_price_per_sq_ft, availability_date,
				property_type, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, id, orgID, p.name, p.street, p.city, p.state, p.postalCode,
			landlordID, p.landlordName, contactID, p.contactName,
			p.totalSqFt, p.availableSqFt, p.status, p.askingPricePerSqFt, p.availabilityDate,
			p.propertyType, now, now)
		if err != nil {
			log.Printf("[Provisioning] Warning: failed to create property %s: %v", p.name, err)
		}
	}
	log.Printf("[Provisioning] Created %d properties", len(properties))

	// ========================================================================
	// LEADS (8 space-seeking leads at various pipeline stages)
	// ========================================================================
	type leadDef struct {
		companyName, firstName, lastName, email, phone string
		status, spaceType, source                      string
		estimatedSqFt                                  int
		estimatedBudget, budgetPerSqFt                 float64
		moveInTimeline                                 string
	}

	leads := []leadDef{
		{
			companyName: "Goldman Sachs Asset Management", firstName: "Rachel", lastName: "Goldberg",
			email: "r.goldberg@gs.com", phone: "212-902-1000",
			status: "Qualified", spaceType: "Office", source: "Referral",
			estimatedSqFt: 75000, estimatedBudget: 4500000, budgetPerSqFt: 60.00,
			moveInTimeline: "Q3 2026",
		},
		{
			companyName: "FedEx Ground Operations", firstName: "Marcus", lastName: "Johnson",
			email: "m.johnson@fedex.com", phone: "901-818-7500",
			status: "Contacted", spaceType: "Industrial", source: "CoStar",
			estimatedSqFt: 250000, estimatedBudget: 3000000, budgetPerSqFt: 12.00,
			moveInTimeline: "Q2 2026",
		},
		{
			companyName: "Blank Rome LLP", firstName: "Susan", lastName: "Hartley",
			email: "s.hartley@blankrome.com", phone: "215-569-5500",
			status: "New", spaceType: "Office", source: "Networking Event",
			estimatedSqFt: 30000, estimatedBudget: 1500000, budgetPerSqFt: 50.00,
			moveInTimeline: "Q4 2026",
		},
		{
			companyName: "Dunkin' Brands Regional", firstName: "Paul", lastName: "Santoro",
			email: "p.santoro@dunkin.com", phone: "781-737-3000",
			status: "Qualified", spaceType: "Retail", source: "LoopNet",
			estimatedSqFt: 2000, estimatedBudget: 120000, budgetPerSqFt: 60.00,
			moveInTimeline: "Q2 2026",
		},
		{
			companyName: "Tesla Service Center", firstName: "Emily", lastName: "Chen",
			email: "e.chen@tesla.com", phone: "650-681-5000",
			status: "Contacted", spaceType: "Industrial", source: "Sign Call",
			estimatedSqFt: 15000, estimatedBudget: 225000, budgetPerSqFt: 15.00,
			moveInTimeline: "Q3 2026",
		},
		{
			companyName: "Deloitte Consulting", firstName: "Andrew", lastName: "Morrison",
			email: "a.morrison@deloitte.com", phone: "212-436-2000",
			status: "Qualified", spaceType: "Office", source: "Referral",
			estimatedSqFt: 120000, estimatedBudget: 7200000, budgetPerSqFt: 60.00,
			moveInTimeline: "Q1 2027",
		},
		{
			companyName: "Aldi US Distribution", firstName: "Frank", lastName: "Bauer",
			email: "f.bauer@aldi.us", phone: "630-879-8100",
			status: "New", spaceType: "Industrial", source: "Cold Call",
			estimatedSqFt: 400000, estimatedBudget: 4800000, budgetPerSqFt: 12.00,
			moveInTimeline: "Q4 2026",
		},
		{
			companyName: "Nuveen Real Estate", firstName: "Claire", lastName: "Fontaine",
			email: "c.fontaine@nuveen.com", phone: "212-323-2000",
			status: "Unqualified", spaceType: "Office", source: "Website",
			estimatedSqFt: 5000, estimatedBudget: 200000, budgetPerSqFt: 40.00,
			moveInTimeline: "No timeline",
		},
	}

	for _, l := range leads {
		id := sfid.NewLead()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO leads (id, org_id, company_name, first_name, last_name, email_address, phone_number,
				status, space_type_needed, estimated_sq_ft, estimated_budget, budget_per_sq_ft,
				move_in_timeline, source, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, id, orgID, l.companyName, l.firstName, l.lastName, l.email, l.phone,
			l.status, l.spaceType, l.estimatedSqFt, l.estimatedBudget, l.budgetPerSqFt,
			l.moveInTimeline, l.source, now, now)
		if err != nil {
			log.Printf("[Provisioning] Warning: failed to create lead %s: %v", l.companyName, err)
		}
	}
	log.Printf("[Provisioning] Created %d leads", len(leads))

	// ========================================================================
	// DEALS (8 deals at various stages with commission tracking)
	// ========================================================================
	type dealDef struct {
		name                                                                 string
		propertyName, tenantName, tenantContactName                         string
		landlordName, landlordContactName                                    string
		leasingBroker, represents, status, dealType                         string
		dealValue, baseRent                                                  float64
		sqFootageLeased                                                      int
		leaseTermLength                                                      string
		commissionPctLandlord, commissionPctTenant                          float64
		expectedCloseDate, closeDate, leaseStartDate, leaseEndDate          string
		commissionStatus                                                     string
		notes                                                                string
	}

	calcCommission := func(dealValue, pct float64) float64 {
		return dealValue * (pct / 100.0)
	}

	deals := []dealDef{
		{
			name:              "WeWork @ One Liberty Plaza",
			propertyName:      "One Liberty Plaza",
			tenantName:        "WeWork Enterprise",
			tenantContactName: "Amanda Torres",
			landlordName:      "Brookfield Property Partners",
			landlordContactName: "Michael Emory",
			leasingBroker: "CBRE Advisory", represents: "Both",
			status: "Closed Won", dealType: "New Lease",
			dealValue: 4200000, baseRent: 350000,
			sqFootageLeased: 85000, leaseTermLength: "10 years",
			commissionPctLandlord: 4.0, commissionPctTenant: 2.0,
			expectedCloseDate: "2025-12-31", closeDate: "2026-01-15",
			leaseStartDate: "2026-03-01", leaseEndDate: "2036-02-28",
			commissionStatus: "Earned",
			notes: "WeWork taking full 12th and 13th floors. Landlord provided 6-month free rent and $45/sqft TI allowance.",
		},
		{
			name:              "Kirkland & Ellis @ Harborside Financial",
			propertyName:      "Harborside Financial Center",
			tenantName:        "Kirkland & Ellis LLP",
			tenantContactName: "James Whitfield",
			landlordName:      "Mack-Cali Realty",
			landlordContactName: "David Sarnoff",
			leasingBroker: "Marcus & Millichap", represents: "Tenant",
			status: "Under Negotiation", dealType: "New Lease",
			dealValue: 2800000, baseRent: 233333,
			sqFootageLeased: 45000, leaseTermLength: "7 years",
			commissionPctLandlord: 0.0, commissionPctTenant: 3.5,
			expectedCloseDate: "2026-03-31", closeDate: "",
			leaseStartDate: "", leaseEndDate: "",
			commissionStatus: "Not Yet Earned",
			notes: "Tenant requires full floor. Negotiating above-standard TI package. Counter-offer submitted 2026-02-20.",
		},
		{
			name:              "Amazon @ Prologis Meadowlands",
			propertyName:      "Prologis Meadowlands",
			tenantName:        "Amazon Last Mile",
			tenantContactName: "Carlos Rivera",
			landlordName:      "Prologis Industrial",
			landlordContactName: "Robert Hamid",
			leasingBroker: "CBRE Advisory", represents: "Landlord",
			status: "Offer Submitted", dealType: "New Lease",
			dealValue: 3500000, baseRent: 291667,
			sqFootageLeased: 200000, leaseTermLength: "7 years",
			commissionPctLandlord: 4.5, commissionPctTenant: 0.0,
			expectedCloseDate: "2026-04-15", closeDate: "",
			leaseStartDate: "", leaseEndDate: "",
			commissionStatus: "Not Yet Earned",
			notes: "Amazon requires 36' clear height and 60 dock doors. Prologis to complete spec build-out. LOI signed 2026-02-01.",
		},
		{
			name:              "Starbucks @ Gateway Center",
			propertyName:      "Gateway Center",
			tenantName:        "Starbucks Regional",
			tenantContactName: "Lisa Nguyen",
			landlordName:      "Mack-Cali Realty",
			landlordContactName: "Patricia Okoye",
			leasingBroker: "Marcus & Millichap", represents: "Both",
			status: "Pipeline", dealType: "New Lease",
			dealValue: 175000, baseRent: 14583,
			sqFootageLeased: 3500, leaseTermLength: "5 years",
			commissionPctLandlord: 5.0, commissionPctTenant: 3.0,
			expectedCloseDate: "2026-05-30", closeDate: "",
			leaseStartDate: "", leaseEndDate: "",
			commissionStatus: "Not Yet Earned",
			notes: "Starbucks ground floor retail space. Corner unit preferred. Reviewing build-out requirements.",
		},
		{
			name:              "Regus @ 225 West Wacker",
			propertyName:      "225 West Wacker Drive",
			tenantName:        "Regus Flex",
			tenantContactName: "Thomas Blake",
			landlordName:      "Brookfield Property Partners",
			landlordContactName: "Sarah Chen",
			leasingBroker: "CBRE Advisory", represents: "Both",
			status: "Closed Won", dealType: "New Lease",
			dealValue: 1500000, baseRent: 125000,
			sqFootageLeased: 25000, leaseTermLength: "5 years",
			commissionPctLandlord: 4.0, commissionPctTenant: 2.0,
			expectedCloseDate: "2025-11-30", closeDate: "2025-12-01",
			leaseStartDate: "2026-01-01", leaseEndDate: "2030-12-31",
			commissionStatus: "Paid",
			notes: "Regus operating coworking space on floors 8. Full commission paid Q1 2026.",
		},
		{
			name:              "Tech Startup @ Gateway Center",
			propertyName:      "Gateway Center",
			tenantName:        "WeWork Enterprise",
			tenantContactName: "Amanda Torres",
			landlordName:      "Mack-Cali Realty",
			landlordContactName: "David Sarnoff",
			leasingBroker: "Marcus & Millichap", represents: "Tenant",
			status: "Closed Lost", dealType: "New Lease",
			dealValue: 400000, baseRent: 33333,
			sqFootageLeased: 8000, leaseTermLength: "3 years",
			commissionPctLandlord: 0.0, commissionPctTenant: 3.0,
			expectedCloseDate: "2026-01-31", closeDate: "2026-01-31",
			leaseStartDate: "", leaseEndDate: "",
			commissionStatus: "Not Yet Earned",
			notes: "Tenant selected alternate location in Manhattan. Budget constraints drove decision.",
		},
		{
			name:              "FedEx @ Prologis O'Hare",
			propertyName:      "Prologis O'Hare Logistics Center",
			tenantName:        "Amazon Last Mile",
			tenantContactName: "Carlos Rivera",
			landlordName:      "Prologis Industrial",
			landlordContactName: "Robert Hamid",
			leasingBroker: "CBRE Advisory", represents: "Landlord",
			status: "Pipeline", dealType: "New Lease",
			dealValue: 1800000, baseRent: 150000,
			sqFootageLeased: 150000, leaseTermLength: "7 years",
			commissionPctLandlord: 4.0, commissionPctTenant: 0.0,
			expectedCloseDate: "2026-06-30", closeDate: "",
			leaseStartDate: "", leaseEndDate: "",
			commissionStatus: "Not Yet Earned",
			notes: "Amazon evaluating O'Hare location for last-mile distribution hub serving Chicago metro.",
		},
		{
			name:              "Kirkland & Ellis Expansion @ One Liberty",
			propertyName:      "One Liberty Plaza",
			tenantName:        "Kirkland & Ellis LLP",
			tenantContactName: "Nicole Park",
			landlordName:      "Brookfield Property Partners",
			landlordContactName: "Michael Emory",
			leasingBroker: "Marcus & Millichap", represents: "Both",
			status: "Pending", dealType: "Expansion",
			dealValue: 950000, baseRent: 79167,
			sqFootageLeased: 18000, leaseTermLength: "3 years",
			commissionPctLandlord: 4.0, commissionPctTenant: 2.5,
			expectedCloseDate: "2026-04-30", closeDate: "",
			leaseStartDate: "", leaseEndDate: "",
			commissionStatus: "Not Yet Earned",
			notes: "K&E expanding to additional half-floor adjacent to their existing space. Expansion right exercised.",
		},
	}

	for _, d := range deals {
		id := sfid.New("0De")
		propertyID := propertyIDs[d.propertyName]
		tenantID := accountIDs[d.tenantName]
		tenantContactID := contactIDs[d.tenantContactName]
		landlordID := accountIDs[d.landlordName]
		landlordContactID := contactIDs[d.landlordContactName]

		commissionAmtLandlord := calcCommission(d.dealValue, d.commissionPctLandlord)
		commissionAmtTenant := calcCommission(d.dealValue, d.commissionPctTenant)
		totalCommission := commissionAmtLandlord + commissionAmtTenant

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO deals (id, org_id, name,
				property_id, property_id_name,
				tenant_id, tenant_id_name,
				tenant_contact_id, tenant_contact_id_name,
				landlord_id, landlord_id_name,
				landlord_contact_id, landlord_contact_id_name,
				leasing_broker, represents, deal_value, sq_footage_leased,
				lease_term_length, base_rent,
				commission_pct_landlord, commission_amt_landlord,
				commission_pct_tenant, commission_amt_tenant,
				total_commission, status, close_date, expected_close_date,
				deal_type, lease_start_date, lease_end_date,
				commission_status, notes,
				created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, id, orgID, d.name,
			propertyID, d.propertyName,
			tenantID, d.tenantName,
			tenantContactID, d.tenantContactName,
			landlordID, d.landlordName,
			landlordContactID, d.landlordContactName,
			d.leasingBroker, d.represents, d.dealValue, d.sqFootageLeased,
			d.leaseTermLength, d.baseRent,
			d.commissionPctLandlord, commissionAmtLandlord,
			d.commissionPctTenant, commissionAmtTenant,
			totalCommission, d.status, d.closeDate, d.expectedCloseDate,
			d.dealType, d.leaseStartDate, d.leaseEndDate,
			d.commissionStatus, d.notes,
			now, now)
		if err != nil {
			log.Printf("[Provisioning] Warning: failed to create deal %s: %v", d.name, err)
		}
	}
	log.Printf("[Provisioning] Created %d deals", len(deals))

	log.Printf("[Provisioning] CRE Broker sample data complete: %d accounts, %d contacts, %d properties, %d leads, %d deals",
		len(accounts), len(contacts), len(properties), len(leads), len(deals))
}
