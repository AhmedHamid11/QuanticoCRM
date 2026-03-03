package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fastcrm/backend/internal/sfid"
)

// ProvisionWulfRecruiting creates the complete Wulf Recruiting CRM setup
// This includes entities, fields, layouts, navigation, and bearings for:
// - Client (recruiting clients)
// - ClientContact (contacts at client companies)
// - Candidate (people being recruited)
// - JobOpening (job orders/JO's)
// - Submittal (pipeline tracking)
// - Activity (activity log)
// - Invoice (billing)
func (s *ProvisioningService) ProvisionWulfRecruiting(ctx context.Context, orgID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Starting Wulf Recruiting provisioning for org %s", orgID)

	// ========================================================================
	// ENTITIES
	// ========================================================================
	entities := []struct{ name, plural string }{
		{"Client", "Clients"},
		{"ClientContact", "Client Contacts"},
		{"Candidate", "Candidates"},
		{"JobOpening", "Job Openings"},
		{"Submittal", "Submittals"},
		{"Activity", "Activities"},
		{"Invoice", "Invoices"},
	}
	for _, e := range entities {
		if err := s.createRecruitingEntity(ctx, orgID, e.name, e.plural, now); err != nil {
			log.Printf("[Provisioning] Warning: failed to create %s entity: %v", e.name, err)
		} else {
			log.Printf("[Provisioning] Created entity %s", e.name)
		}
	}

	// ========================================================================
	// CLIENT FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Client", "name", "Company Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Client", "industry", "Industry", "varchar", false, 2, now)
	s.createField(ctx, orgID, "Client", "website", "Website", "url", false, 3, now)
	s.createField(ctx, orgID, "Client", "phoneNumber", "Phone", "phone", false, 4, now)
	s.createField(ctx, orgID, "Client", "emailAddress", "Email", "email", false, 5, now)
	s.createField(ctx, orgID, "Client", "addressStreet", "Street", "varchar", false, 10, now)
	s.createField(ctx, orgID, "Client", "addressCity", "City", "varchar", false, 11, now)
	s.createField(ctx, orgID, "Client", "addressState", "State", "varchar", false, 12, now)
	s.createField(ctx, orgID, "Client", "addressCountry", "Country", "varchar", false, 13, now)
	s.createField(ctx, orgID, "Client", "addressPostalCode", "Postal Code", "varchar", false, 14, now)
	s.createField(ctx, orgID, "Client", "contractTerms", "Contract Terms", "varchar", false, 20, now)
	s.createField(ctx, orgID, "Client", "contractSignedDate", "Contract Signed", "date", false, 21, now)
	s.createField(ctx, orgID, "Client", "clientSince", "Client Since", "date", false, 22, now)
	s.createEnumField(ctx, orgID, "Client", "status", "Status", []string{"Active", "Inactive", "Prospect"}, 30, now)
	s.createField(ctx, orgID, "Client", "accountManager", "Account Manager", "varchar", false, 31, now)
	s.createField(ctx, orgID, "Client", "openingsSummary", "Typical Openings", "text", false, 40, now)
	s.createField(ctx, orgID, "Client", "notes", "Notes", "text", false, 50, now)
	s.createField(ctx, orgID, "Client", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Client", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// CLIENT CONTACT FIELDS
	// ========================================================================
	s.createLinkField(ctx, orgID, "ClientContact", "clientId", "Client", "Client", 1, now)
	s.createField(ctx, orgID, "ClientContact", "firstName", "First Name", "varchar", true, 2, now)
	s.createField(ctx, orgID, "ClientContact", "lastName", "Last Name", "varchar", true, 3, now)
	s.createField(ctx, orgID, "ClientContact", "role", "Role/Title", "varchar", false, 4, now)
	s.createField(ctx, orgID, "ClientContact", "email", "Email", "email", false, 5, now)
	s.createField(ctx, orgID, "ClientContact", "phone", "Phone", "phone", false, 6, now)
	s.createField(ctx, orgID, "ClientContact", "isPrimary", "Primary Contact", "bool", false, 7, now)
	s.createField(ctx, orgID, "ClientContact", "notes", "Notes", "text", false, 10, now)
	s.createField(ctx, orgID, "ClientContact", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "ClientContact", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// CANDIDATE FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Candidate", "firstName", "First Name", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Candidate", "lastName", "Last Name", "varchar", true, 2, now)
	s.createField(ctx, orgID, "Candidate", "email", "Email", "email", false, 3, now)
	s.createField(ctx, orgID, "Candidate", "phone", "Phone", "phone", false, 4, now)
	s.createEnumField(ctx, orgID, "Candidate", "phoneType", "Phone Type", []string{"Mobile", "Home", "Work"}, 5, now)
	s.createField(ctx, orgID, "Candidate", "addressCity", "City", "varchar", false, 10, now)
	s.createField(ctx, orgID, "Candidate", "addressState", "State", "varchar", false, 11, now)
	s.createField(ctx, orgID, "Candidate", "addressCountry", "Country", "varchar", false, 12, now)
	s.createField(ctx, orgID, "Candidate", "willingToRelocate", "Willing to Relocate", "bool", false, 20, now)
	s.createField(ctx, orgID, "Candidate", "relocationAreas", "Relocation Areas", "varchar", false, 21, now)
	s.createField(ctx, orgID, "Candidate", "geoRange", "Geographic Range", "varchar", false, 22, now)
	s.createField(ctx, orgID, "Candidate", "currentSalary", "Current Salary", "varchar", false, 30, now)
	s.createField(ctx, orgID, "Candidate", "currentBonus", "Current Bonus", "varchar", false, 31, now)
	s.createField(ctx, orgID, "Candidate", "salaryExpectations", "Salary Expectations", "varchar", false, 32, now)
	s.createField(ctx, orgID, "Candidate", "currentEmployer", "Current Employer", "varchar", false, 40, now)
	s.createField(ctx, orgID, "Candidate", "currentTitle", "Current Title", "varchar", false, 41, now)
	s.createField(ctx, orgID, "Candidate", "positionType", "Target Position Type", "varchar", false, 42, now)
	s.createField(ctx, orgID, "Candidate", "industryExperience", "Industry Experience", "varchar", false, 43, now)
	s.createField(ctx, orgID, "Candidate", "yearsExperience", "Years Experience", "int", false, 44, now)
	s.createEnumField(ctx, orgID, "Candidate", "status", "Status", []string{"Active", "Placed", "Inactive", "Do Not Contact"}, 50, now)
	s.createField(ctx, orgID, "Candidate", "isPlaceable", "Placeable", "bool", false, 51, now)
	s.createField(ctx, orgID, "Candidate", "resumeUrl", "Resume URL", "url", false, 60, now)
	s.createField(ctx, orgID, "Candidate", "source", "Source", "varchar", false, 70, now)
	s.createField(ctx, orgID, "Candidate", "sourceDate", "Source Date", "date", false, 71, now)
	s.createField(ctx, orgID, "Candidate", "lastContactedDate", "Last Contacted", "date", false, 72, now)
	s.createField(ctx, orgID, "Candidate", "notes", "Notes", "text", false, 80, now)
	s.createField(ctx, orgID, "Candidate", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Candidate", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// JOB OPENING FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "JobOpening", "joNumber", "JO #", "varchar", true, 1, now)
	s.createField(ctx, orgID, "JobOpening", "title", "Job Title", "varchar", true, 2, now)
	s.createLinkField(ctx, orgID, "JobOpening", "clientId", "Client", "Client", 3, now)
	s.createField(ctx, orgID, "JobOpening", "hiringManager", "Hiring Manager", "varchar", false, 4, now)
	s.createEnumField(ctx, orgID, "JobOpening", "category", "Priority", []string{"AA", "A", "B", "C", "D"}, 5, now)
	s.createField(ctx, orgID, "JobOpening", "city", "City", "varchar", false, 10, now)
	s.createField(ctx, orgID, "JobOpening", "state", "State", "varchar", false, 11, now)
	s.createField(ctx, orgID, "JobOpening", "country", "Country", "varchar", false, 12, now)
	s.createEnumField(ctx, orgID, "JobOpening", "workType", "Work Type", []string{"On-site", "Remote", "Hybrid"}, 13, now)
	s.createField(ctx, orgID, "JobOpening", "salaryRange", "Salary Range", "varchar", false, 20, now)
	s.createField(ctx, orgID, "JobOpening", "bonusInfo", "Bonus Info", "varchar", false, 21, now)
	s.createEnumField(ctx, orgID, "JobOpening", "status", "Status", []string{"Open", "On Hold", "Filled", "Cancelled"}, 30, now)
	s.createField(ctx, orgID, "JobOpening", "datePosted", "Date Posted", "date", false, 31, now)
	s.createField(ctx, orgID, "JobOpening", "dateFilled", "Date Filled", "date", false, 32, now)
	s.createField(ctx, orgID, "JobOpening", "owner", "Owner", "varchar", false, 40, now)
	s.createField(ctx, orgID, "JobOpening", "submittalsTotal", "Submittals", "int", false, 41, now)
	s.createField(ctx, orgID, "JobOpening", "description", "Description", "text", false, 50, now)
	s.createField(ctx, orgID, "JobOpening", "notes", "Notes", "text", false, 51, now)
	s.createField(ctx, orgID, "JobOpening", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "JobOpening", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// SUBMITTAL (PIPELINE) FIELDS
	// ========================================================================
	s.createLinkField(ctx, orgID, "Submittal", "candidateId", "Candidate", "Candidate", 1, now)
	s.createLinkField(ctx, orgID, "Submittal", "jobOpeningId", "Job Opening", "JobOpening", 2, now)
	s.createLinkField(ctx, orgID, "Submittal", "clientId", "Client", "Client", 3, now)
	s.createField(ctx, orgID, "Submittal", "joNumber", "JO #", "varchar", false, 4, now)
	s.createField(ctx, orgID, "Submittal", "recruiter", "Recruiter", "varchar", false, 5, now)
	s.createEnumField(ctx, orgID, "Submittal", "stage", "Stage", []string{
		"Submitted", "PI_1", "PI_2", "PI_3", "Onsite_1", "Onsite_2", "Offer", "Accepted", "Started", "Placed",
	}, 10, now)
	s.createField(ctx, orgID, "Submittal", "submittedDate", "Submitted", "date", false, 20, now)
	s.createField(ctx, orgID, "Submittal", "pi1Date", "Phone Interview 1", "date", false, 21, now)
	s.createField(ctx, orgID, "Submittal", "pi2Date", "Phone Interview 2", "date", false, 22, now)
	s.createField(ctx, orgID, "Submittal", "pi3Date", "Phone Interview 3", "date", false, 23, now)
	s.createField(ctx, orgID, "Submittal", "onsite1Date", "On-site 1", "date", false, 24, now)
	s.createField(ctx, orgID, "Submittal", "onsite2Date", "On-site 2", "date", false, 25, now)
	s.createField(ctx, orgID, "Submittal", "offerDate", "Offer", "date", false, 26, now)
	s.createField(ctx, orgID, "Submittal", "acceptedDate", "Accepted", "date", false, 27, now)
	s.createField(ctx, orgID, "Submittal", "startDate", "Start Date", "date", false, 28, now)
	s.createField(ctx, orgID, "Submittal", "finalSalary", "Final Salary", "varchar", false, 30, now)
	s.createField(ctx, orgID, "Submittal", "commissionAmount", "Commission", "currency", false, 31, now)
	s.createField(ctx, orgID, "Submittal", "pipelineDays", "Pipeline Days", "int", false, 32, now)
	s.createField(ctx, orgID, "Submittal", "invoiceDate", "Invoice Date", "date", false, 40, now)
	s.createField(ctx, orgID, "Submittal", "invoiceDueDate", "Invoice Due", "date", false, 41, now)
	s.createField(ctx, orgID, "Submittal", "paidDate", "Paid Date", "date", false, 42, now)
	s.createEnumField(ctx, orgID, "Submittal", "paidStatus", "Payment Status", []string{"Pending", "Paid", "Overdue"}, 43, now)
	s.createField(ctx, orgID, "Submittal", "recruiterPayout", "Recruiter Payout", "currency", false, 44, now)
	s.createField(ctx, orgID, "Submittal", "feedback", "Feedback", "text", false, 50, now)
	s.createField(ctx, orgID, "Submittal", "notes", "Notes", "text", false, 51, now)
	s.createField(ctx, orgID, "Submittal", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Submittal", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// ACTIVITY FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Activity", "parentType", "Related To Type", "varchar", true, 1, now)
	s.createField(ctx, orgID, "Activity", "parentId", "Related To ID", "varchar", true, 2, now)
	s.createField(ctx, orgID, "Activity", "parentName", "Related To", "varchar", false, 3, now)
	s.createEnumField(ctx, orgID, "Activity", "activityType", "Type", []string{
		"Call", "Email", "Note", "Meeting", "LinkedIn", "Text", "Voicemail",
	}, 4, now)
	s.createField(ctx, orgID, "Activity", "subject", "Subject", "varchar", false, 5, now)
	s.createField(ctx, orgID, "Activity", "description", "Description", "text", false, 6, now)
	s.createField(ctx, orgID, "Activity", "activityDate", "Date", "date", true, 7, now)
	s.createField(ctx, orgID, "Activity", "createdBy", "Created By", "varchar", false, 8, now)
	s.createField(ctx, orgID, "Activity", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Activity", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// INVOICE FIELDS
	// ========================================================================
	s.createField(ctx, orgID, "Invoice", "invoiceNumber", "Invoice #", "varchar", true, 1, now)
	s.createLinkField(ctx, orgID, "Invoice", "clientId", "Client", "Client", 2, now)
	s.createLinkField(ctx, orgID, "Invoice", "candidateId", "Candidate", "Candidate", 3, now)
	s.createLinkField(ctx, orgID, "Invoice", "jobOpeningId", "Job Opening", "JobOpening", 4, now)
	s.createField(ctx, orgID, "Invoice", "positionTitle", "Position", "varchar", false, 5, now)
	s.createField(ctx, orgID, "Invoice", "hiredDate", "Hired Date", "date", false, 10, now)
	s.createField(ctx, orgID, "Invoice", "invoiceDate", "Invoice Date", "date", false, 11, now)
	s.createField(ctx, orgID, "Invoice", "dueDate", "Due Date", "date", false, 12, now)
	s.createField(ctx, orgID, "Invoice", "paidDate", "Paid Date", "date", false, 13, now)
	s.createField(ctx, orgID, "Invoice", "baseSalary", "Base Salary", "currency", false, 20, now)
	s.createField(ctx, orgID, "Invoice", "feePercentage", "Fee %", "float", false, 21, now)
	s.createField(ctx, orgID, "Invoice", "feeAmount", "Fee Amount", "currency", false, 22, now)
	s.createEnumField(ctx, orgID, "Invoice", "status", "Status", []string{
		"Draft", "Sent", "Paid", "Overdue", "Cancelled",
	}, 30, now)
	s.createField(ctx, orgID, "Invoice", "recruiterPayout", "Recruiter Payout", "currency", false, 40, now)
	s.createField(ctx, orgID, "Invoice", "payoutDate", "Payout Date", "date", false, 41, now)
	s.createField(ctx, orgID, "Invoice", "notes", "Notes", "text", false, 50, now)
	s.createField(ctx, orgID, "Invoice", "createdAt", "Created At", "datetime", false, 100, now)
	s.createField(ctx, orgID, "Invoice", "modifiedAt", "Modified At", "datetime", false, 101, now)

	// ========================================================================
	// LIST LAYOUTS
	// ========================================================================
	s.createLayout(ctx, orgID, "Client", "list",
		`["name","industry","status","accountManager","contractTerms","clientSince","phoneNumber","createdAt"]`, now)
	s.createLayout(ctx, orgID, "ClientContact", "list",
		`["firstName","lastName","clientId","role","email","phone","isPrimary"]`, now)
	s.createLayout(ctx, orgID, "Candidate", "list",
		`["firstName","lastName","currentTitle","currentEmployer","addressCity","addressState","status","salaryExpectations","lastContactedDate"]`, now)
	s.createLayout(ctx, orgID, "JobOpening", "list",
		`["joNumber","title","clientId","category","city","state","salaryRange","status","owner","datePosted"]`, now)
	s.createLayout(ctx, orgID, "Submittal", "list",
		`["candidateId","jobOpeningId","joNumber","clientName","stage","recruiter","submittedDate","startDate","finalSalary","commissionAmount"]`, now)
	s.createLayout(ctx, orgID, "Activity", "list",
		`["activityDate","activityType","parentName","subject","createdBy"]`, now)
	s.createLayout(ctx, orgID, "Invoice", "list",
		`["invoiceNumber","clientId","candidateId","positionTitle","baseSalary","feeAmount","status","invoiceDate","dueDate","paidDate"]`, now)

	// ========================================================================
	// DETAIL LAYOUTS
	// ========================================================================

	// Client detail layout
	s.createLayout(ctx, orgID, "Client", "detail", `[
		{"label":"Company Info","rows":[
			[{"field":"name"},{"field":"industry"}],
			[{"field":"website"},{"field":"phoneNumber"}],
			[{"field":"emailAddress"}]
		]},
		{"label":"Address","rows":[
			[{"field":"addressStreet"}],
			[{"field":"addressCity"},{"field":"addressState"}],
			[{"field":"addressPostalCode"},{"field":"addressCountry"}]
		]},
		{"label":"Contract","rows":[
			[{"field":"contractTerms"},{"field":"contractSignedDate"}],
			[{"field":"clientSince"},{"field":"status"}],
			[{"field":"accountManager"}]
		]},
		{"label":"Notes","rows":[
			[{"field":"openingsSummary"}],
			[{"field":"notes"}]
		]}
	]`, now)

	// ClientContact detail layout
	s.createLayout(ctx, orgID, "ClientContact", "detail", `[
		{"label":"Contact Info","rows":[
			[{"field":"firstName"},{"field":"lastName"}],
			[{"field":"role"},{"field":"clientId"}],
			[{"field":"email"},{"field":"phone"}],
			[{"field":"isPrimary"}]
		]},
		{"label":"Notes","rows":[
			[{"field":"notes"}]
		]}
	]`, now)

	// Candidate detail layout
	s.createLayout(ctx, orgID, "Candidate", "detail", `[
		{"label":"Basic Info","rows":[
			[{"field":"firstName"},{"field":"lastName"}],
			[{"field":"email"},{"field":"phone"}],
			[{"field":"phoneType"},{"field":"status"}],
			[{"field":"isPlaceable"}]
		]},
		{"label":"Location","rows":[
			[{"field":"addressCity"},{"field":"addressState"}],
			[{"field":"addressCountry"},{"field":"geoRange"}],
			[{"field":"willingToRelocate"},{"field":"relocationAreas"}]
		]},
		{"label":"Professional","rows":[
			[{"field":"currentEmployer"},{"field":"currentTitle"}],
			[{"field":"positionType"},{"field":"industryExperience"}],
			[{"field":"yearsExperience"}]
		]},
		{"label":"Compensation","rows":[
			[{"field":"currentSalary"},{"field":"currentBonus"}],
			[{"field":"salaryExpectations"}]
		]},
		{"label":"Source & Contact","rows":[
			[{"field":"source"},{"field":"sourceDate"}],
			[{"field":"lastContactedDate"}],
			[{"field":"resumeUrl"}]
		]},
		{"label":"Notes","rows":[
			[{"field":"notes"}]
		]}
	]`, now)

	// JobOpening detail layout
	s.createLayout(ctx, orgID, "JobOpening", "detail", `[
		{"label":"Job Info","rows":[
			[{"field":"joNumber"},{"field":"title"}],
			[{"field":"clientId"},{"field":"hiringManager"}],
			[{"field":"category"},{"field":"status"}],
			[{"field":"owner"}]
		]},
		{"label":"Location","rows":[
			[{"field":"city"},{"field":"state"}],
			[{"field":"country"},{"field":"workType"}]
		]},
		{"label":"Compensation","rows":[
			[{"field":"salaryRange"}],
			[{"field":"bonusInfo"}]
		]},
		{"label":"Dates","rows":[
			[{"field":"datePosted"},{"field":"dateFilled"}],
			[{"field":"submittalsTotal"}]
		]},
		{"label":"Details","rows":[
			[{"field":"description"}],
			[{"field":"notes"}]
		]}
	]`, now)

	// Submittal detail layout (Pipeline tracking)
	s.createLayout(ctx, orgID, "Submittal", "detail", `[
		{"label":"Submittal Info","rows":[
			[{"field":"candidateId"},{"field":"jobOpeningId"}],
			[{"field":"joNumber"},{"field":"clientName"}],
			[{"field":"recruiter"},{"field":"stage"}]
		]},
		{"label":"Pipeline Dates","rows":[
			[{"field":"submittedDate"},{"field":"pi1Date"}],
			[{"field":"pi2Date"},{"field":"pi3Date"}],
			[{"field":"onsite1Date"},{"field":"onsite2Date"}],
			[{"field":"offerDate"},{"field":"acceptedDate"}],
			[{"field":"startDate"},{"field":"pipelineDays"}]
		]},
		{"label":"Compensation","rows":[
			[{"field":"finalSalary"},{"field":"commissionAmount"}]
		]},
		{"label":"Billing","rows":[
			[{"field":"invoiceDate"},{"field":"invoiceDueDate"}],
			[{"field":"paidDate"},{"field":"paidStatus"}],
			[{"field":"recruiterPayout"}]
		]},
		{"label":"Notes","rows":[
			[{"field":"feedback"}],
			[{"field":"notes"}]
		]}
	]`, now)

	// Activity detail layout
	s.createLayout(ctx, orgID, "Activity", "detail", `[
		{"label":"Activity","rows":[
			[{"field":"activityType"},{"field":"activityDate"}],
			[{"field":"parentType"},{"field":"parentName"}],
			[{"field":"subject"}],
			[{"field":"description"}],
			[{"field":"createdBy"}]
		]}
	]`, now)

	// Invoice detail layout
	s.createLayout(ctx, orgID, "Invoice", "detail", `[
		{"label":"Invoice Info","rows":[
			[{"field":"invoiceNumber"},{"field":"status"}],
			[{"field":"clientId"},{"field":"candidateId"}],
			[{"field":"jobOpeningId"},{"field":"positionTitle"}]
		]},
		{"label":"Dates","rows":[
			[{"field":"hiredDate"},{"field":"invoiceDate"}],
			[{"field":"dueDate"},{"field":"paidDate"}]
		]},
		{"label":"Amounts","rows":[
			[{"field":"baseSalary"},{"field":"feePercentage"}],
			[{"field":"feeAmount"}]
		]},
		{"label":"Payout","rows":[
			[{"field":"recruiterPayout"},{"field":"payoutDate"}]
		]},
		{"label":"Notes","rows":[
			[{"field":"notes"}]
		]}
	]`, now)

	// ========================================================================
	// NAVIGATION TABS — replace any existing tabs with Wulf-specific tabs
	// ========================================================================
	_, _ = s.db.ExecContext(ctx, `DELETE FROM navigation_tabs WHERE org_id = ?`, orgID)
	s.createNavTab(ctx, orgID, "Home", "/", "", 0, true, now)
	s.createNavTab(ctx, orgID, "Clients", "/clients", "Client", 1, true, now)
	s.createNavTab(ctx, orgID, "Candidates", "/candidates", "Candidate", 2, true, now)
	s.createNavTab(ctx, orgID, "Job Openings", "/job-openings", "JobOpening", 3, true, now)
	s.createNavTab(ctx, orgID, "Pipeline", "/submittals", "Submittal", 4, true, now)
	s.createNavTab(ctx, orgID, "Invoices", "/invoices", "Invoice", 5, false, now)
	s.createNavTab(ctx, orgID, "Activities", "/activities", "Activity", 6, false, now)

	// ========================================================================
	// BEARINGS (Stage Progress Indicators)
	// ========================================================================
	s.createBearing(ctx, orgID, "Submittal", "Pipeline Stage", "stage", 1, now)
	s.createBearing(ctx, orgID, "JobOpening", "JO Status", "status", 1, now)
	s.createBearing(ctx, orgID, "Invoice", "Invoice Status", "status", 1, now)
	s.createBearing(ctx, orgID, "Client", "Client Status", "status", 1, now)
	s.createBearing(ctx, orgID, "Candidate", "Candidate Status", "status", 1, now)

	log.Printf("[Provisioning] Completed Wulf Recruiting provisioning for org %s", orgID)
	return nil
}

// createRecruitingEntity creates an entity with recruiting-specific lookup config
func (s *ProvisioningService) createRecruitingEntity(ctx context.Context, orgID, name, plural, now string) error {
	id := sfid.New("0Et")

	displayField, searchFields := getRecruitingEntityLookupConfig(name)

	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO entity_defs (id, org_id, name, label, label_plural, icon, color, is_custom, is_customizable, has_stream, has_activities, display_field, search_fields, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, '', '', 0, 1, 0, 0, ?, ?, ?, ?)
	`, id, orgID, name, name, plural, displayField, searchFields, now, now)
	return err
}

// getRecruitingEntityLookupConfig returns display_field and search_fields for recruiting entities
func getRecruitingEntityLookupConfig(entityName string) (displayField string, searchFields string) {
	switch entityName {
	case "Client":
		return "name", `["name", "industry"]`
	case "ClientContact":
		return "first_name || ' ' || last_name", `["first_name", "last_name", "email"]`
	case "Candidate":
		return "first_name || ' ' || last_name", `["first_name", "last_name", "email", "current_employer"]`
	case "JobOpening":
		return "jo_number || ' - ' || title", `["jo_number", "title", "client_name"]`
	case "Submittal":
		return "candidate_id_name || ' → ' || job_opening_id_name", `["candidate_id_name", "job_opening_id_name", "jo_number", "client_name"]`
	case "Activity":
		return "subject", `["subject", "description"]`
	case "Invoice":
		return "invoice_number", `["invoice_number", "client_name", "candidate_name"]`
	default:
		return "name", `["name"]`
	}
}

// ProvisionWulfRecruitingSampleData imports sample data from the spreadsheets
func (s *ProvisioningService) ProvisionWulfRecruitingSampleData(ctx context.Context, orgID string) {
	now := time.Now().UTC().Format(time.RFC3339)
	log.Printf("[Provisioning] Creating Wulf Recruiting sample data for org %s", orgID)

	// ========================================================================
	// SAMPLE CLIENTS (from Clients sheet)
	// ========================================================================
	clients := []struct {
		name, contact1, contact1Role, contact1Info, contact2, contact2Role, industry, status, contractSigned string
	}{
		{"Andritz (LDX)", "Scott Terhune", "CRO/Capital Sales VP", "", "Bill O'Shea", "CEO", "APC", "Active", ""},
		{"Vezer", "Desi Delgado", "President", "", "", "", "Cement Mtce", "Prospect", ""},
		{"Cidan", "Michael Posson", "VP Aftermarket", "", "Steph Greiner", "HR", "", "Active", "2026-01-23"},
		{"Hive LLC", "Nick", "", "", "", "", "Smart Home tech", "Prospect", ""},
		{"Techtron", "Ed Baldwin", "President", "", "Sean", "Training Director", "", "Active", "2021-01-01"},
		{"Aumund Corp", "Wes Allen", "Prez Ops & Aftermarket", "", "", "", "", "Prospect", ""},
		{"AVC", "Joelle", "HR", "", "", "", "Veterinarian", "Active", ""},
		{"RIE Coatings", "John Warne", "CEO", "", "", "", "", "Inactive", ""},
		{"Absolent", "Darin Dullum", "Absolent", "darin@dullum.net", "", "", "", "Prospect", ""},
		{"Amrize", "Allen Greer", "Prez", "alan.greer@amrize.com", "Kimberly Mickan", "HR", "Cement", "Prospect", ""},
		{"Turnell Corp", "Luciano Bodero", "Projects Director", "314-630-9151", "", "", "Heavy industry", "Prospect", ""},
	}

	clientIDs := make(map[string]string)
	for _, c := range clients {
		clientID := sfid.New("0Cl")
		clientIDs[c.name] = clientID
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO clients (id, org_id, name, industry, status, contract_signed_date, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, clientID, orgID, c.name, c.industry, c.status, c.contractSigned, now, now)
		if err != nil {
			log.Printf("Warning: failed to create client %s: %v", c.name, err)
		}

		// Create primary contact
		if c.contact1 != "" {
			names := splitName(c.contact1)
			contactID := sfid.New("0Cc")
			_, err := s.db.ExecContext(ctx, `
				INSERT INTO client_contacts (id, org_id, client_id, client_name, first_name, last_name, role, email, is_primary, created_at, modified_at, deleted, custom_fields)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, 0, '{}')
			`, contactID, orgID, clientID, c.name, names[0], names[1], c.contact1Role, c.contact1Info, now, now)
			if err != nil {
				log.Printf("Warning: failed to create contact %s: %v", c.contact1, err)
			}
		}

		// Create secondary contact
		if c.contact2 != "" {
			names := splitName(c.contact2)
			contactID := sfid.New("0Cc")
			_, err := s.db.ExecContext(ctx, `
				INSERT INTO client_contacts (id, org_id, client_id, client_name, first_name, last_name, role, is_primary, created_at, modified_at, deleted, custom_fields)
				VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, 0, '{}')
			`, contactID, orgID, clientID, c.name, names[0], names[1], c.contact2Role, now, now)
			if err != nil {
				log.Printf("Warning: failed to create contact %s: %v", c.contact2, err)
			}
		}
	}

	// ========================================================================
	// SAMPLE JOB OPENINGS (from Working JO's sheet)
	// ========================================================================
	jobOpenings := []struct {
		joNumber, company, title, city, state, salaryRange, category, status, owner, notes string
		datePosted                                                                         string
	}{
		{"1251", "LDX/Andritz", "APC Sales Manager", "Remote", "US", "$140-$160K + bonus (TBD)", "A", "Open", "Lone Wulf", "", "2026-01-06"},
		{"1249", "Techtron", "Mobile Lab Tech/IH", "Minneapolis/St. Paul metro", "MN", "$55-$60K + bonus", "AA", "Open", "Lone Wulf", "6 weeks training in Anoka (paid)", "2026-01-06"},
		{"1250", "Techtron", "Mobile Lab Tech/IH", "Minneapolis/St. Paul metro", "MN", "$55-$60K + bonus", "AA", "Open", "Lone Wulf", "6 weeks training in Anoka (paid)", "2026-01-06"},
		{"1251", "AVC", "Veterinarian", "Glendale", "CA", "$190K + bonus & $20K sign-on bonus", "C", "On Hold", "Lone Wulf", "Hold until March 2026", "2025-05-24"},
		{"1252", "AVC", "Veterinarian", "Whittier", "CA", "$190K + bonus & $20K sign-on bonus", "C", "Open", "Lone Wulf", "3-10 pm", "2025-05-24"},
		{"1253", "AVC", "Veterinarian", "Silverlake", "CA", "$190K + bonus & $20K sign-on bonus", "C", "Open", "Lone Wulf", "3-10 pm", "2025-05-24"},
		{"1254", "Cidan Machinery", "Service Technician", "Various", "US", "$30-$40/hr", "A", "Open", "Lone Wulf", "", "2026-01-23"},
	}

	joIDs := make(map[string]string)
	for _, jo := range jobOpenings {
		joID := sfid.New("0Jo")
		joIDs[jo.joNumber] = joID
		clientID := clientIDs[jo.company]
		if clientID == "" {
			clientID = clientIDs["Techtron"] // Fallback
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO job_openings (id, org_id, jo_number, title, client_id, client_name, city, state, salary_range, category, status, owner, notes, date_posted, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, joID, orgID, jo.joNumber, jo.title, clientID, jo.company, jo.city, jo.state, jo.salaryRange, jo.category, jo.status, jo.owner, jo.notes, jo.datePosted, now, now)
		if err != nil {
			log.Printf("Warning: failed to create JO %s: %v", jo.joNumber, err)
		}
	}

	// ========================================================================
	// SAMPLE CANDIDATES (from Placeable Candidates sheet)
	// ========================================================================
	candidates := []struct {
		firstName, lastName, positionType, currentEmployer, preferredLocation, salary string
	}{
		{"Derek", "Richards", "Aggregates Production/Plant Mgr/mining ops", "Imerys", "Redding, CA, other specific areas", ""},
		{"Francesco", "Zana", "Capital Projects", "", "", "$180+"},
		{"Ali", "Imran", "CRO/Shift Super", "unemployed, has green card", "", ""},
		{"Bob", "Hamilton", "EHS Manager", "Lhoist", "New Braunfels area", ""},
		{"Krunal", "Darjee", "Electrical", "Electrical Consultants", "South Jersey/Philly area.", ""},
		{"Sam", "Santana", "Electrical", "", "upstate NY", ""},
		{"Cordero", "Benitez", "Electrical Engineer", "MMM", "Central TX", "$80K or so"},
		{"Craig", "Lawrence", "Engineering/Mtce/Rel. Mgr", "Keystone", "Nazareth, York, PA", "$105K"},
		{"Brian", "Males", "Env Mgr", "CPC", "Florida #1, TX or CO", ""},
		{"Brandon", "Blue", "Environmental Director", "", "", ""},
		{"Shelby", "Olsen", "Environmental Director", "", "", ""},
		{"Mike", "Williams", "Fleet Manager (Maint)", "", "Open", ""},
		{"Juan", "Hanna", "GM", "Saga Matrix", "East", "$200K"},
		{"Brent", "Phelps", "Logistics, Procurement", "AZ, unemployed", "", "$135K last salary"},
		{"Steven", "Abboud", "Maint Mgr, Plant Mgr", "don't know", "US", ""},
		{"Tim", "Adams", "Maint/Reliability Engineer", "BYK", "San Antonio area pref.", "$104-$115K"},
		{"Chris", "Kerschen", "Maintenance Supervisor", "Argos (prev)", "Warmer better (open)", "was $90K"},
		{"Mike", "Elliot", "Maintenance Engineer", "LafargeHolcim", "Northeast, PA, TX", "$90K"},
		{"Tom", "Kuruvilla", "Maintenance Engineer", "unemployed, has green card", "open", "$80K-ish?"},
		{"Julian", "Vaquera", "Maintenance Engineer", "MM Midlothian", "Texas - central, south only", "$100K + 8-9%"},
	}

	candidateIDs := make(map[string]string)
	for _, c := range candidates {
		candID := sfid.New("0Ca")
		candidateIDs[c.firstName+" "+c.lastName] = candID
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO candidates (id, org_id, first_name, last_name, position_type, current_employer, geo_range, salary_expectations, status, is_placeable, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'Active', 1, ?, ?, 0, '{}')
		`, candID, orgID, c.firstName, c.lastName, c.positionType, c.currentEmployer, c.preferredLocation, c.salary, now, now)
		if err != nil {
			log.Printf("Warning: failed to create candidate %s %s: %v", c.firstName, c.lastName, err)
		}
	}

	// ========================================================================
	// SAMPLE PIPELINE/SUBMITTALS (from Pipeline sheet)
	// ========================================================================
	placements := []struct {
		firstName, lastName, joNumber, title, company, salary string
		commission                                            float64
		recruiter                                             string
		stage                                                 string
	}{
		{"Matt", "Devitt", "1163", "VP Aftermarket", "LDX Solutions", "$200K", 50000, "", "Placed"},
		{"Jacob", "Sylvester", "7185", "Maintenance Manager", "Argos", "$145K", 21750, "", "Placed"},
		{"Perry", "Poradek", "1166", "VP Sales", "RIE Coatings", "$150K", 37500, "", "Placed"},
		{"Alex", "Baskette", "1165", "Project Manager", "LDX Solutions", "$160K", 32000, "", "Placed"},
		{"Zach", "Rowe", "1164", "Mobile Lab Tech (Denver)", "Techtron", "$51.5K", 12875, "", "Placed"},
		{"John", "Gill", "116", "Mobile Lab Tech (Tulsa)", "Techtron", "$55K", 13750, "Kalanen", "Placed"},
		{"Andrew", "Wisner", "1167", "Kiln SME", "NAK", "$55K", 11625, "", "Placed"},
		{"Armando", "Leyva", "7002", "Production Manager", "Eagle", "$145K", 21750, "", "Placed"},
		{"Aimsley", "Kadlec", "1163", "Mobile Lab Tech (Des Moines)", "Techtron", "$55K", 13750, "Kalanen", "Placed"},
		{"Kent", "Hall", "1170", "Engineering Mgr", "LDX Solutions", "$160K", 40000, "", "Placed"},
	}

	for _, p := range placements {
		subID := sfid.New("0Su")
		candID := candidateIDs[p.firstName+" "+p.lastName]
		if candID == "" {
			// Create candidate if doesn't exist
			candID = sfid.New("0Ca")
			candidateIDs[p.firstName+" "+p.lastName] = candID
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO candidates (id, org_id, first_name, last_name, status, is_placeable, created_at, modified_at, deleted, custom_fields)
				VALUES (?, ?, ?, ?, 'Placed', 0, ?, ?, 0, '{}')
			`, candID, orgID, p.firstName, p.lastName, now, now)
		}

		// Look up client_id from company name
		clientID := clientIDs[p.company]

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO submittals (id, org_id, candidate_id, candidate_name, job_opening_id, job_opening_title, jo_number, client_id, client_name, recruiter, stage, final_salary, commission_amount, created_at, modified_at, deleted, custom_fields)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '{}')
		`, subID, orgID, candID, p.firstName+" "+p.lastName, joIDs[p.joNumber], p.title, p.joNumber, clientID, p.company, p.recruiter, p.stage, p.salary, p.commission, now, now)
		if err != nil {
			log.Printf("Warning: failed to create submittal for %s %s: %v", p.firstName, p.lastName, err)
		}
	}

	log.Printf("[Provisioning] Created Wulf Recruiting sample data for org %s", orgID)
}

// splitName splits a name into first and last name
func splitName(name string) [2]string {
	parts := [2]string{name, ""}
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == ' ' {
			parts[0] = name[:i]
			parts[1] = name[i+1:]
			break
		}
	}
	if parts[1] == "" {
		parts[1] = parts[0]
		parts[0] = ""
	}
	return parts
}

// ProvisionWulfRecruitingComplete does full provisioning including sample data
func (s *ProvisioningService) ProvisionWulfRecruitingComplete(ctx context.Context, orgID string) error {
	if err := s.ProvisionWulfRecruiting(ctx, orgID); err != nil {
		return fmt.Errorf("failed to provision metadata: %w", err)
	}
	// Navigation is already created inside ProvisionWulfRecruiting (custom tabs)
	s.ProvisionWulfRecruitingSampleData(ctx, orgID)
	return nil
}
