// Package sfid generates Salesforce-style 18-character IDs.
// Format: PPP + 15 alphanumeric characters
// where PPP is a 3-character entity prefix (e.g., "003" for Contact)
package sfid

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
	"sync"
	"time"
)

// Entity prefixes following Salesforce conventions
const (
	PrefixContact         = "003"
	PrefixAccount         = "001"
	PrefixLead            = "00Q"
	PrefixUser            = "005"
	PrefixTask            = "00T" // Task/Activity
	PrefixFieldDef        = "00N" // Custom field
	PrefixLayout          = "0Lv" // Layout
	PrefixEntity          = "0En" // Entity definition
	PrefixOrg             = "00D" // Organization
	PrefixRelatedList     = "0RL" // Related list config
	PrefixTripwire        = "0Tw" // Tripwire (webhook trigger)
	PrefixTripwireLog     = "0TL" // Tripwire execution log
	PrefixWebhookSettings = "0Ws" // Org webhook settings
	PrefixBearing         = "0Br" // Bearing (stage progress indicator)
	PrefixMembership      = "0Mb" // User-org membership
	PrefixSession         = "0Ss" // User session
	PrefixInvitation      = "0Iv" // Org invitation
	PrefixValidationRule  = "0Vr" // Validation rule
	PrefixCustomPage      = "0Cp" // Custom page
	PrefixAPIToken        = "0At" // API token
	PrefixQuote              = "0Qt" // Quote
	PrefixQuoteLineItem      = "0QL" // Quote line item
	PrefixPdfTemplate        = "0Pt" // PDF template
	PrefixPasswordResetToken = "0Pr" // Password reset token
	PrefixMigrationRun       = "0Mr" // Migration run
	PrefixTokenFamily        = "0Tf" // Token family (for refresh token rotation)
	PrefixAuditLog           = "0Ad" // Audit log entry
	PrefixMergeSnapshot      = "0Ms" // Merge snapshot (for undo capability)
	PrefixScanSchedule       = "0Sc" // Scan schedule
	PrefixScanJob            = "0Sj" // Scan job execution
	PrefixNotification       = "0Nt" // In-app notification
	PrefixSFConnection       = "0Sf" // Salesforce connection
	PrefixSyncJob            = "0Sy" // Sync job
	PrefixSFFieldMapping     = "0Sm" // Salesforce field mapping
	PrefixIngestKey          = "0Ik" // Ingest API key
	PrefixIngestJob          = "0Ij" // Ingest job
	PrefixMirror             = "0Mi" // Mirror config
	PrefixMirrorField        = "0Mf" // Mirror source field
	PrefixDeltaKey           = "0Dk" // Delta key for deduplication
)

// Custom base32 alphabet: 0-9, A-Z excluding I, L, O, U (to avoid confusion)
// This gives us 32 characters for base32 encoding
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var encoding = base32.NewEncoding(alphabet).WithPadding(base32.NoPadding)

var (
	counter   uint32
	counterMu sync.Mutex
	lastTime  int64
)

// New generates a new 18-character Salesforce-style ID for the given entity prefix.
// The ID format is: PPP (3 char prefix) + 15 alphanumeric characters
// The 15 characters encode timestamp + counter + random data for uniqueness.
func New(prefix string) string {
	if len(prefix) != 3 {
		prefix = "000"
	}

	// Get current timestamp in milliseconds (42 bits = ~139 years)
	now := time.Now().UnixMilli()

	// Increment counter for same-millisecond uniqueness
	counterMu.Lock()
	if now == lastTime {
		counter++
	} else {
		counter = 0
		lastTime = now
	}
	currentCounter := counter
	counterMu.Unlock()

	// Build a 9-byte buffer:
	// - 5 bytes for timestamp (40 bits, good for ~34 years)
	// - 2 bytes for counter (16 bits, 65536 per ms)
	// - 2 bytes for randomness
	buf := make([]byte, 9)

	// Encode timestamp (5 bytes, big-endian)
	buf[0] = byte(now >> 32)
	buf[1] = byte(now >> 24)
	buf[2] = byte(now >> 16)
	buf[3] = byte(now >> 8)
	buf[4] = byte(now)

	// Encode counter (2 bytes)
	buf[5] = byte(currentCounter >> 8)
	buf[6] = byte(currentCounter)

	// Add randomness (2 bytes)
	rand.Read(buf[7:9])

	// Base32 encode to get 15 characters (9 bytes * 8 bits / 5 bits per char = 14.4, rounds to 15)
	encoded := encoding.EncodeToString(buf)

	// Ensure exactly 15 characters
	if len(encoded) > 15 {
		encoded = encoded[:15]
	} else if len(encoded) < 15 {
		encoded = encoded + strings.Repeat("0", 15-len(encoded))
	}

	return prefix + encoded
}

// NewContact generates a new Contact ID (prefix: 003)
func NewContact() string {
	return New(PrefixContact)
}

// NewAccount generates a new Account ID (prefix: 001)
func NewAccount() string {
	return New(PrefixAccount)
}

// NewLead generates a new Lead ID (prefix: 00Q)
func NewLead() string {
	return New(PrefixLead)
}

// NewUser generates a new User ID (prefix: 005)
func NewUser() string {
	return New(PrefixUser)
}

// NewFieldDef generates a new Field Definition ID (prefix: 00N)
func NewFieldDef() string {
	return New(PrefixFieldDef)
}

// NewLayout generates a new Layout ID (prefix: 0Lv)
func NewLayout() string {
	return New(PrefixLayout)
}

// NewEntity generates a new Entity Definition ID (prefix: 0En)
func NewEntity() string {
	return New(PrefixEntity)
}

// NewOrg generates a new Organization ID (prefix: 00D)
func NewOrg() string {
	return New(PrefixOrg)
}

// NewRelatedList generates a new Related List Config ID (prefix: 0RL)
func NewRelatedList() string {
	return New(PrefixRelatedList)
}

// NewTask generates a new Task ID (prefix: 00T)
func NewTask() string {
	return New(PrefixTask)
}

// NewTripwire generates a new Tripwire ID (prefix: 0Tw)
func NewTripwire() string {
	return New(PrefixTripwire)
}

// NewTripwireLog generates a new Tripwire Log ID (prefix: 0TL)
func NewTripwireLog() string {
	return New(PrefixTripwireLog)
}

// NewWebhookSettings generates a new Webhook Settings ID (prefix: 0Ws)
func NewWebhookSettings() string {
	return New(PrefixWebhookSettings)
}

// NewBearing generates a new Bearing Config ID (prefix: 0Br)
func NewBearing() string {
	return New(PrefixBearing)
}

// GetPrefix extracts the 3-character prefix from an ID
func GetPrefix(id string) string {
	if len(id) < 3 {
		return ""
	}
	return id[:3]
}

// IsContact checks if the ID is a Contact ID
func IsContact(id string) bool {
	return GetPrefix(id) == PrefixContact
}

// IsAccount checks if the ID is an Account ID
func IsAccount(id string) bool {
	return GetPrefix(id) == PrefixAccount
}

// IsTask checks if the ID is a Task ID
func IsTask(id string) bool {
	return GetPrefix(id) == PrefixTask
}

// IsValid checks if an ID has the correct format (18 chars, valid prefix)
func IsValid(id string) bool {
	return len(id) == 18
}

// NewMembership generates a new Membership ID (prefix: 0Mb)
func NewMembership() string {
	return New(PrefixMembership)
}

// NewSession generates a new Session ID (prefix: 0Ss)
func NewSession() string {
	return New(PrefixSession)
}

// NewInvitation generates a new Invitation ID (prefix: 0Iv)
func NewInvitation() string {
	return New(PrefixInvitation)
}

// NewValidationRule generates a new Validation Rule ID (prefix: 0Vr)
func NewValidationRule() string {
	return New(PrefixValidationRule)
}

// NewCustomPage generates a new Custom Page ID (prefix: 0Cp)
func NewCustomPage() string {
	return New(PrefixCustomPage)
}

// NewAPIToken generates a new API Token ID (prefix: 0At)
func NewAPIToken() string {
	return New(PrefixAPIToken)
}

// NewQuote generates a new Quote ID (prefix: 0Qt)
func NewQuote() string {
	return New(PrefixQuote)
}

// NewQuoteLineItem generates a new Quote Line Item ID (prefix: 0QL)
func NewQuoteLineItem() string {
	return New(PrefixQuoteLineItem)
}

// NewPdfTemplate generates a new PDF Template ID (prefix: 0Pt)
func NewPdfTemplate() string {
	return New(PrefixPdfTemplate)
}

// NewPasswordResetToken generates a new Password Reset Token ID (prefix: 0Pr)
func NewPasswordResetToken() string {
	return New(PrefixPasswordResetToken)
}

// NewTokenFamily generates a new Token Family ID (prefix: 0Tf)
// Used to group refresh tokens from the same login session for reuse detection
func NewTokenFamily() string {
	return New(PrefixTokenFamily)
}

// NewAuditLog generates a new Audit Log Entry ID (prefix: 0Ad)
func NewAuditLog() string {
	return New(PrefixAuditLog)
}

// NewMergeSnapshot generates a new Merge Snapshot ID (prefix: 0Ms)
func NewMergeSnapshot() string {
	return New(PrefixMergeSnapshot)
}

// NewScanSchedule generates a new Scan Schedule ID (prefix: 0Sc)
func NewScanSchedule() string {
	return New(PrefixScanSchedule)
}

// NewScanJob generates a new Scan Job ID (prefix: 0Sj)
func NewScanJob() string {
	return New(PrefixScanJob)
}

// NewNotification generates a new Notification ID (prefix: 0Nt)
func NewNotification() string {
	return New(PrefixNotification)
}

// NewSFConnection generates a new Salesforce Connection ID (prefix: 0Sf)
func NewSFConnection() string {
	return New(PrefixSFConnection)
}

// NewSyncJob generates a new Sync Job ID (prefix: 0Sy)
func NewSyncJob() string {
	return New(PrefixSyncJob)
}

// NewSFFieldMapping generates a new Salesforce Field Mapping ID (prefix: 0Sm)
func NewSFFieldMapping() string {
	return New(PrefixSFFieldMapping)
}

// NewIngestKey generates a new Ingest API Key ID (prefix: 0Ik)
func NewIngestKey() string {
	return New(PrefixIngestKey)
}

// NewIngestJob generates a new Ingest Job ID (prefix: 0Ij)
func NewIngestJob() string {
	return New(PrefixIngestJob)
}

// NewMirror generates a new Mirror Config ID (prefix: 0Mi)
func NewMirror() string {
	return New(PrefixMirror)
}

// NewMirrorField generates a new Mirror Source Field ID (prefix: 0Mf)
func NewMirrorField() string {
	return New(PrefixMirrorField)
}

// NewDeltaKey generates a new Delta Key ID (prefix: 0Dk)
func NewDeltaKey() string {
	return New(PrefixDeltaKey)
}
