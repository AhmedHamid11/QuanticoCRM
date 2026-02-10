package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/fastcrm/backend/internal/sfid"
	_ "github.com/mattn/go-sqlite3"
)

const recruitingOrgID = "00DKFC4GC5S000CA70"
const recruitingUserID = "005KFC4GC5Y0000F30"

func main() {
	db, err := sql.Open("sqlite3", "/Users/ahmedhamid/Documents/FastCRM/FastCRM/fastcrm/fastcrm.db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	ctx := context.Background()
	rand.Seed(time.Now().UnixNano())

	log.Println("Seeding Recruitment Agency with comprehensive data...")

	// Create client companies (accounts)
	accountIDs := createAccounts(ctx, db)
	log.Printf("Created %d client accounts", len(accountIDs))

	// Create candidates (prospects)
	prospectIDs := createProspects(ctx, db)
	log.Printf("Created %d candidates", len(prospectIDs))

	// Create job openings
	jobIDs := createJobs(ctx, db, accountIDs)
	log.Printf("Created %d jobs", len(jobIDs))

	// Create submissions linking candidates to jobs
	submissionCount := createSubmissions(ctx, db, prospectIDs, jobIDs)
	log.Printf("Created %d submissions", submissionCount)

	log.Println("Recruitment Agency seed completed successfully!")
}

func createAccounts(ctx context.Context, db *sql.DB) []string {
	now := time.Now().UTC().Format(time.RFC3339)

	accounts := []struct {
		name     string
		website  string
		industry string
		accType  string
		city     string
		state    string
	}{
		{"TechVenture Labs", "https://techventurelabs.com", "Technology", "Client", "San Francisco", "CA"},
		{"DataCore Systems", "https://datacoresystems.com", "Technology", "Client", "Austin", "TX"},
		{"CloudFirst Inc", "https://cloudfirst.io", "Technology", "Client", "Seattle", "WA"},
		{"FinanceFlow Partners", "https://financeflow.com", "Finance", "Client", "New York", "NY"},
		{"MedTech Innovations", "https://medtechinnovations.com", "Healthcare", "Client", "Boston", "MA"},
		{"GreenEnergy Corp", "https://greenenergycorp.com", "Energy", "Client", "Denver", "CO"},
		{"RetailMax Holdings", "https://retailmax.com", "Retail", "Client", "Chicago", "IL"},
		{"CyberShield Security", "https://cybershieldsec.com", "Technology", "Client", "Washington", "DC"},
		{"AI Dynamics", "https://aidynamics.ai", "Technology", "Client", "Palo Alto", "CA"},
		{"BioPharm Research", "https://biopharmresearch.com", "Healthcare", "Client", "San Diego", "CA"},
		{"QuantumLeap Technologies", "https://quantumleaptech.com", "Technology", "Client", "Portland", "OR"},
		{"NexGen Consulting", "https://nexgenconsulting.com", "Consulting", "Client", "Atlanta", "GA"},
		{"SmartHome Solutions", "https://smarthomesol.com", "Technology", "Client", "Phoenix", "AZ"},
		{"EduTech Academy", "https://edutechacademy.com", "Education", "Client", "Nashville", "TN"},
		{"LogiTrans Inc", "https://logitrans.com", "Logistics", "Client", "Dallas", "TX"},
	}

	var ids []string
	for _, a := range accounts {
		id := sfid.New("001")
		ids = append(ids, id)

		_, err := db.ExecContext(ctx, `
			INSERT INTO accounts (id, org_id, name, website, industry, type, billing_address_city, billing_address_state, billing_address_country, created_at, modified_at, created_by_id, modified_by_id, deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
		`, id, recruitingOrgID, a.name, a.website, a.industry, a.accType, a.city, a.state, "USA", now, now, recruitingUserID, recruitingUserID)
		if err != nil {
			log.Printf("Warning: Failed to create account %s: %v", a.name, err)
		}
	}
	return ids
}

func createProspects(ctx context.Context, db *sql.DB) []string {
	now := time.Now().UTC().Format(time.RFC3339)

	candidates := []struct {
		firstName      string
		lastName       string
		email          string
		phone          string
		currentTitle   string
		currentCompany string
		yearsExp       int
		desiredSalary  float64
		skills         string
		status         string
		source         string
		linkedIn       string
	}{
		{"Sarah", "Chen", "sarah.chen@email.com", "415-555-0101", "Senior Software Engineer", "Google", 7, 180000, "Python,Go,Kubernetes,AWS,Machine Learning", "Active", "LinkedIn", "linkedin.com/in/sarahchen"},
		{"Michael", "Rodriguez", "m.rodriguez@email.com", "512-555-0102", "Full Stack Developer", "Meta", 5, 165000, "React,Node.js,TypeScript,PostgreSQL,GraphQL", "Active", "Referral", "linkedin.com/in/mrodriguez"},
		{"Emily", "Johnson", "emily.j@email.com", "206-555-0103", "DevOps Engineer", "Amazon", 6, 175000, "AWS,Terraform,Docker,Jenkins,Python", "Active", "LinkedIn", "linkedin.com/in/emilyjohnson"},
		{"David", "Kim", "david.kim@email.com", "650-555-0104", "Data Scientist", "Netflix", 4, 170000, "Python,TensorFlow,SQL,Spark,Machine Learning", "Active", "Job Board", "linkedin.com/in/davidkim"},
		{"Jessica", "Williams", "jwilliams@email.com", "617-555-0105", "Product Manager", "Salesforce", 8, 200000, "Agile,JIRA,Product Strategy,Data Analysis,SQL", "Interview", "LinkedIn", "linkedin.com/in/jessicawilliams"},
		{"James", "Thompson", "james.t@email.com", "212-555-0106", "Backend Engineer", "Bloomberg", 6, 185000, "Java,Spring Boot,Microservices,Kafka,Redis", "Active", "Referral", "linkedin.com/in/jamesthompson"},
		{"Amanda", "Garcia", "a.garcia@email.com", "303-555-0107", "Frontend Developer", "Airbnb", 4, 155000, "React,Vue.js,CSS,TypeScript,Webpack", "Placed", "Job Board", "linkedin.com/in/amandagarcia"},
		{"Robert", "Lee", "robert.lee@email.com", "408-555-0108", "Engineering Manager", "Apple", 10, 250000, "Leadership,Agile,System Design,Python,Go", "Passive", "LinkedIn", "linkedin.com/in/robertlee"},
		{"Michelle", "Brown", "m.brown@email.com", "619-555-0109", "ML Engineer", "Nvidia", 5, 195000, "PyTorch,CUDA,Python,Computer Vision,Deep Learning", "Active", "Referral", "linkedin.com/in/michellebrown"},
		{"Christopher", "Davis", "c.davis@email.com", "503-555-0110", "SRE", "Dropbox", 7, 180000, "Linux,Kubernetes,Prometheus,Go,Python", "Interview", "LinkedIn", "linkedin.com/in/christopherdavis"},
		{"Jennifer", "Martinez", "j.martinez@email.com", "404-555-0111", "Security Engineer", "CrowdStrike", 6, 190000, "Security,Python,AWS,Penetration Testing,SIEM", "Active", "Job Board", "linkedin.com/in/jennifermartinez"},
		{"Daniel", "Anderson", "d.anderson@email.com", "480-555-0112", "iOS Developer", "Uber", 5, 165000, "Swift,Objective-C,iOS,UIKit,SwiftUI", "Active", "LinkedIn", "linkedin.com/in/danielanderson"},
		{"Lauren", "Taylor", "l.taylor@email.com", "615-555-0113", "QA Engineer", "Microsoft", 4, 135000, "Selenium,Python,TestNG,API Testing,CI/CD", "Placed", "Referral", "linkedin.com/in/laurentaylor"},
		{"Matthew", "Wilson", "m.wilson@email.com", "214-555-0114", "Cloud Architect", "Oracle", 9, 220000, "AWS,Azure,GCP,Terraform,Architecture", "Passive", "LinkedIn", "linkedin.com/in/matthewwilson"},
		{"Ashley", "Moore", "a.moore@email.com", "469-555-0115", "Data Engineer", "Spotify", 5, 170000, "Python,Spark,Airflow,SQL,Snowflake", "Active", "Job Board", "linkedin.com/in/ashleymoore"},
		{"Kevin", "Jackson", "k.jackson@email.com", "312-555-0116", "Tech Lead", "Stripe", 8, 230000, "Java,Scala,Distributed Systems,Leadership,Architecture", "Interview", "Referral", "linkedin.com/in/kevinjackson"},
		{"Nicole", "White", "n.white@email.com", "858-555-0117", "Bioinformatics Scientist", "Illumina", 6, 160000, "Python,R,Genomics,Machine Learning,Statistics", "Active", "LinkedIn", "linkedin.com/in/nicolewhite"},
		{"Brandon", "Harris", "b.harris@email.com", "720-555-0118", "Platform Engineer", "Palantir", 5, 185000, "Go,Kubernetes,gRPC,PostgreSQL,Docker", "Active", "Job Board", "linkedin.com/in/brandonharris"},
		{"Stephanie", "Clark", "s.clark@email.com", "425-555-0119", "UX Designer", "Adobe", 6, 145000, "Figma,User Research,Prototyping,CSS,Design Systems", "Placed", "LinkedIn", "linkedin.com/in/stephanieclark"},
		{"Andrew", "Lewis", "a.lewis@email.com", "978-555-0120", "Embedded Systems Engineer", "Tesla", 7, 175000, "C,C++,RTOS,Linux,Embedded Systems", "Active", "Referral", "linkedin.com/in/andrewlewis"},
		{"Rachel", "Robinson", "r.robinson@email.com", "949-555-0121", "Senior React Developer", "Twitch", 5, 170000, "React,Redux,TypeScript,Node.js,GraphQL", "Active", "LinkedIn", "linkedin.com/in/rachelrobinson"},
		{"Justin", "Walker", "j.walker@email.com", "202-555-0122", "Cybersecurity Analyst", "Booz Allen", 4, 140000, "SIEM,Incident Response,Python,Network Security,SOC", "Active", "Job Board", "linkedin.com/in/justinwalker"},
		{"Megan", "Hall", "m.hall@email.com", "415-555-0123", "AI Research Scientist", "OpenAI", 6, 280000, "Python,PyTorch,NLP,Transformers,Research", "Passive", "Referral", "linkedin.com/in/meganhall"},
		{"Tyler", "Allen", "t.allen@email.com", "512-555-0124", "Blockchain Developer", "Coinbase", 4, 200000, "Solidity,Ethereum,Web3,JavaScript,Smart Contracts", "Active", "LinkedIn", "linkedin.com/in/tylerallen"},
		{"Samantha", "Young", "s.young@email.com", "206-555-0125", "Technical Program Manager", "Amazon", 7, 195000, "Program Management,Agile,Technical Planning,SQL,Communication", "Interview", "Referral", "linkedin.com/in/samanthayoung"},
	}

	var ids []string
	for _, c := range candidates {
		id := sfid.New("Rec")
		ids = append(ids, id)
		name := fmt.Sprintf("%s %s", c.firstName, c.lastName)

		_, err := db.ExecContext(ctx, `
			INSERT INTO prospects (id, name, first_name, last_name, email, phone, current_title, current_company, years_experience, desired_salary, skills, status, source, linked_in_url, stage, org_id, created_at, modified_at, created_by_id, modified_by_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, name, c.firstName, c.lastName, c.email, c.phone, c.currentTitle, c.currentCompany, c.yearsExp, c.desiredSalary, c.skills, c.status, c.source, c.linkedIn, "New", recruitingOrgID, now, now, recruitingUserID, recruitingUserID)
		if err != nil {
			log.Printf("Warning: Failed to create prospect %s: %v", name, err)
		}
	}
	return ids
}

func createJobs(ctx context.Context, db *sql.DB, accountIDs []string) []string {
	now := time.Now().UTC().Format(time.RFC3339)

	jobs := []struct {
		name           string
		accountIdx     int
		jobType        string
		workLocation   string
		department     string
		location       string
		salaryMin      float64
		salaryMax      float64
		status         string
		priority       string
		openings       int
		skills         string
		description    string
	}{
		{"Senior Backend Engineer", 0, "Full-Time", "Hybrid", "Engineering", "San Francisco, CA", 170000, 220000, "Open", "High", 2, "Go,Python,Kubernetes,PostgreSQL", "Build scalable backend services for our growing platform"},
		{"Staff Frontend Engineer", 0, "Full-Time", "Remote", "Engineering", "Remote USA", 180000, 240000, "Open", "High", 1, "React,TypeScript,GraphQL,Performance", "Lead frontend architecture and mentor junior developers"},
		{"Data Engineer", 1, "Full-Time", "On-site", "Data", "Austin, TX", 150000, 190000, "Open", "Medium", 2, "Python,Spark,Airflow,SQL,AWS", "Build and maintain data pipelines and infrastructure"},
		{"DevOps Engineer", 2, "Full-Time", "Hybrid", "Platform", "Seattle, WA", 160000, 200000, "Open", "High", 1, "AWS,Terraform,Kubernetes,CI/CD", "Manage cloud infrastructure and deployment pipelines"},
		{"ML Engineer", 2, "Full-Time", "Remote", "AI/ML", "Remote USA", 180000, 250000, "Open", "High", 2, "Python,PyTorch,MLOps,AWS SageMaker", "Deploy and scale machine learning models in production"},
		{"Senior Financial Analyst", 3, "Full-Time", "On-site", "Finance", "New York, NY", 120000, 160000, "Open", "Medium", 1, "Excel,SQL,Financial Modeling,Python", "Analyze financial data and support decision making"},
		{"Quantitative Developer", 3, "Full-Time", "Hybrid", "Technology", "New York, NY", 200000, 300000, "Open", "High", 1, "Python,C++,Mathematics,Statistics", "Develop quantitative trading strategies"},
		{"Healthcare Data Scientist", 4, "Full-Time", "Hybrid", "Data Science", "Boston, MA", 140000, 180000, "Open", "Medium", 1, "Python,R,Healthcare,Machine Learning,Statistics", "Analyze healthcare data to improve patient outcomes"},
		{"Biomedical Engineer", 4, "Full-Time", "On-site", "R&D", "Boston, MA", 130000, 170000, "Open", "Medium", 2, "Medical Devices,Regulatory,CAD,Python", "Design and develop medical devices"},
		{"Sustainability Engineer", 5, "Full-Time", "Hybrid", "Engineering", "Denver, CO", 110000, 150000, "Open", "Low", 1, "Renewable Energy,Data Analysis,Project Management", "Support sustainability initiatives and analysis"},
		{"E-commerce Platform Lead", 6, "Full-Time", "Hybrid", "Technology", "Chicago, IL", 170000, 220000, "Open", "High", 1, "Java,Microservices,E-commerce,Leadership", "Lead the e-commerce platform team"},
		{"Security Engineer", 7, "Full-Time", "On-site", "Security", "Washington, DC", 160000, 210000, "Open", "High", 2, "Cybersecurity,Penetration Testing,SIEM,Python", "Protect systems and respond to security incidents"},
		{"AI Research Scientist", 8, "Full-Time", "Hybrid", "Research", "Palo Alto, CA", 200000, 350000, "Open", "High", 1, "PhD,Deep Learning,NLP,Publications,PyTorch", "Conduct cutting-edge AI research"},
		{"Computer Vision Engineer", 8, "Full-Time", "Remote", "AI/ML", "Remote USA", 170000, 230000, "Open", "Medium", 1, "Python,OpenCV,PyTorch,Computer Vision", "Develop computer vision applications"},
		{"Clinical Data Manager", 9, "Full-Time", "On-site", "Clinical", "San Diego, CA", 100000, 130000, "Open", "Medium", 2, "Clinical Trials,SAS,Data Management,FDA", "Manage clinical trial data and ensure compliance"},
		{"Bioinformatics Developer", 9, "Full-Time", "Hybrid", "R&D", "San Diego, CA", 140000, 180000, "Open", "Medium", 1, "Python,Genomics,AWS,Pipeline Development", "Build genomics analysis pipelines"},
		{"Platform Engineer", 10, "Full-Time", "Remote", "Engineering", "Portland, OR", 150000, 190000, "Filled", "Medium", 0, "Go,Kubernetes,gRPC,PostgreSQL", "Build and maintain internal developer platform"},
		{"Solutions Architect", 11, "Full-Time", "Remote", "Consulting", "Atlanta, GA", 160000, 200000, "Open", "High", 2, "AWS,Architecture,Client-facing,Technical Sales", "Design solutions and support sales process"},
		{"IoT Developer", 12, "Full-Time", "On-site", "Engineering", "Phoenix, AZ", 130000, 170000, "Open", "Medium", 1, "IoT,Python,MQTT,Embedded,Cloud", "Develop IoT solutions for smart home products"},
		{"EdTech Product Manager", 13, "Full-Time", "Hybrid", "Product", "Nashville, TN", 120000, 160000, "Open", "Medium", 1, "Product Management,Education,Agile,Analytics", "Lead product strategy for education platform"},
		{"Supply Chain Engineer", 14, "Full-Time", "On-site", "Operations", "Dallas, TX", 100000, 140000, "Open", "Low", 1, "Supply Chain,Data Analysis,Python,Optimization", "Optimize logistics and supply chain operations"},
		{"VP of Engineering", 0, "Full-Time", "Hybrid", "Leadership", "San Francisco, CA", 350000, 450000, "Open", "Critical", 1, "Leadership,Architecture,Scaling,Strategy", "Lead engineering organization and technical strategy"},
		{"Junior Software Engineer", 1, "Full-Time", "On-site", "Engineering", "Austin, TX", 90000, 120000, "Open", "Low", 3, "Python,JavaScript,SQL,Git", "Entry-level engineering role with mentorship"},
		{"Site Reliability Engineer", 2, "Full-Time", "Remote", "Platform", "Remote USA", 170000, 210000, "On Hold", "Medium", 1, "Linux,Kubernetes,Monitoring,Go,Python", "Ensure system reliability and performance"},
	}

	var ids []string
	for _, j := range jobs {
		id := sfid.New("Job")
		ids = append(ids, id)

		accountID := ""
		accountName := ""
		if j.accountIdx < len(accountIDs) {
			accountID = accountIDs[j.accountIdx]
			// Get account name
			var name string
			err := db.QueryRowContext(ctx, "SELECT name FROM accounts WHERE id = ?", accountID).Scan(&name)
			if err == nil {
				accountName = name
			}
		}

		openDate := time.Now().AddDate(0, 0, -rand.Intn(30)).Format("2006-01-02")
		targetClose := time.Now().AddDate(0, 0, 30+rand.Intn(60)).Format("2006-01-02")

		_, err := db.ExecContext(ctx, `
			INSERT INTO jobs (id, name, account_id_id, account_id_name, job_type, work_location, department, location, salary_min, salary_max, status, priority, number_of_openings, required_skills, description, open_date, target_close_date, org_id, created_at, modified_at, created_by_id, modified_by_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, j.name, accountID, accountName, j.jobType, j.workLocation, j.department, j.location, j.salaryMin, j.salaryMax, j.status, j.priority, j.openings, j.skills, j.description, openDate, targetClose, recruitingOrgID, now, now, recruitingUserID, recruitingUserID)
		if err != nil {
			log.Printf("Warning: Failed to create job %s: %v", j.name, err)
		}
	}
	return ids
}

func createSubmissions(ctx context.Context, db *sql.DB, prospectIDs, jobIDs []string) int {
	now := time.Now().UTC().Format(time.RFC3339)

	rejectionReasons := []string{"Not enough experience", "Salary expectations too high", "Cultural fit concerns", "Failed technical interview", "Position filled", "Candidate withdrew", "Better qualified candidate"}

	// Create targeted submissions - matching candidates with relevant jobs
	submissions := []struct {
		prospectIdx int
		jobIdx      int
		status      string
		stage       string
		daysAgo     int
		offerAmount float64
	}{
		// Sarah Chen -> Backend Engineer
		{0, 0, "Interview Complete", "Technical Interview", 5, 0},
		// Michael Rodriguez -> Frontend Engineer
		{1, 1, "Offer Extended", "Offer", 3, 195000},
		// Emily Johnson -> DevOps Engineer
		{2, 3, "Interview Scheduled", "Technical Interview", 2, 0},
		// David Kim -> ML Engineer
		{3, 4, "Submitted to Client", "Initial Screen", 7, 0},
		{3, 12, "Screening", "Initial Screen", 4, 0},
		// Jessica Williams -> Technical Program Manager
		{24, 19, "Offer Accepted", "Closed", 1, 190000},
		// James Thompson -> Backend roles
		{5, 0, "Interview Complete", "Hiring Manager Interview", 8, 0},
		{5, 2, "Rejected", "Technical Interview", 15, 0},
		// Amanda Garcia -> Frontend (Placed)
		{6, 1, "Placed", "Closed", 20, 175000},
		// Robert Lee -> VP Engineering
		{7, 21, "Screening", "Initial Screen", 3, 0},
		// Michelle Brown -> ML/AI roles
		{8, 4, "Interview Scheduled", "Technical Interview", 1, 0},
		{8, 12, "Submitted to Client", "Initial Screen", 6, 0},
		// Christopher Davis -> SRE
		{9, 23, "Interview Complete", "Hiring Manager Interview", 4, 0},
		// Jennifer Martinez -> Security Engineer
		{10, 11, "Offer Extended", "Offer", 2, 185000},
		// Daniel Anderson -> Not in our current jobs - generic submission
		{11, 18, "Screening", "Initial Screen", 5, 0},
		// Lauren Taylor -> QA (Placed)
		{12, 22, "Placed", "Closed", 30, 110000},
		// Matthew Wilson -> Solutions Architect
		{13, 17, "Interview Scheduled", "Initial Screen", 6, 0},
		// Ashley Moore -> Data Engineer
		{14, 2, "Submitted to Client", "Initial Screen", 4, 0},
		// Kevin Jackson -> E-commerce Lead
		{15, 10, "Interview Complete", "Final Round", 3, 0},
		// Nicole White -> Bioinformatics
		{16, 15, "Offer Extended", "Offer", 1, 165000},
		// Brandon Harris -> Platform Engineer
		{17, 16, "Placed", "Closed", 45, 175000},
		// Stephanie Clark -> Not a direct fit but submitted
		{18, 19, "Rejected", "Initial Screen", 20, 0},
		// Andrew Lewis -> IoT Developer
		{19, 18, "Interview Scheduled", "Technical Interview", 7, 0},
		// Rachel Robinson -> Frontend
		{20, 1, "Screening", "Initial Screen", 2, 0},
		// Justin Walker -> Security
		{21, 11, "Submitted to Client", "Initial Screen", 8, 0},
		// Megan Hall -> AI Research
		{22, 12, "Interview Complete", "Final Round", 5, 0},
		// Tyler Allen -> Not direct fit
		{23, 0, "Withdrawn", "Technical Interview", 12, 0},
		// Samantha Young -> EdTech PM
		{24, 19, "Rejected", "Hiring Manager Interview", 25, 0},
		// Additional cross-submissions for active jobs
		{0, 2, "New", "Initial Screen", 1, 0},
		{1, 0, "Screening", "Initial Screen", 3, 0},
		{2, 4, "Rejected", "Technical Interview", 10, 0},
		{3, 7, "Submitted to Client", "Initial Screen", 6, 0},
		{4, 17, "Interview Scheduled", "Technical Interview", 4, 0},
		{5, 10, "Screening", "Initial Screen", 2, 0},
		{8, 13, "New", "Initial Screen", 0, 0},
		{9, 3, "Interview Complete", "Hiring Manager Interview", 7, 0},
		{10, 7, "Submitted to Client", "Initial Screen", 5, 0},
		{14, 4, "Interview Scheduled", "Technical Interview", 3, 0},
	}

	count := 0
	for _, s := range submissions {
		if s.prospectIdx >= len(prospectIDs) || s.jobIdx >= len(jobIDs) {
			continue
		}

		id := sfid.New("Sub")
		prospectID := prospectIDs[s.prospectIdx]
		jobID := jobIDs[s.jobIdx]

		// Get names for display
		var prospectName, jobName string
		db.QueryRowContext(ctx, "SELECT name FROM prospects WHERE id = ?", prospectID).Scan(&prospectName)
		db.QueryRowContext(ctx, "SELECT name FROM jobs WHERE id = ?", jobID).Scan(&jobName)

		name := fmt.Sprintf("%s - %s", prospectName, jobName)
		submissionDate := time.Now().AddDate(0, 0, -s.daysAgo).Format("2006-01-02")

		var interviewDate, startDate, rejectionReason, clientFeedback sql.NullString
		var offerAmount, placementFee sql.NullFloat64

		// Set fields based on status
		if s.status == "Interview Scheduled" || s.status == "Interview Complete" {
			interviewDate.String = time.Now().AddDate(0, 0, -s.daysAgo+2).Format("2006-01-02")
			interviewDate.Valid = true
		}
		if s.status == "Offer Extended" || s.status == "Offer Accepted" || s.status == "Placed" {
			offerAmount.Float64 = s.offerAmount
			offerAmount.Valid = s.offerAmount > 0
			if s.status == "Placed" {
				startDate.String = time.Now().AddDate(0, 0, -s.daysAgo+14).Format("2006-01-02")
				startDate.Valid = true
				placementFee.Float64 = s.offerAmount * 0.20 // 20% placement fee
				placementFee.Valid = true
			}
		}
		if s.status == "Rejected" {
			rejectionReason.String = rejectionReasons[rand.Intn(len(rejectionReasons))]
			rejectionReason.Valid = true
		}
		if s.status == "Interview Complete" || s.status == "Offer Extended" {
			feedbacks := []string{
				"Strong technical skills, good cultural fit",
				"Impressive experience, would like second round",
				"Solid candidate, moving to final round",
				"Great communication skills",
				"Very knowledgeable, team liked them",
			}
			clientFeedback.String = feedbacks[rand.Intn(len(feedbacks))]
			clientFeedback.Valid = true
		}

		_, err := db.ExecContext(ctx, `
			INSERT INTO submissions (id, name, prospect_id_id, prospect_id_name, job_id_id, job_id_name, submission_date, status, stage, interview_date, offer_amount, start_date, placement_fee, rejection_reason, client_feedback, org_id, created_at, modified_at, created_by_id, modified_by_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, name, prospectID, prospectName, jobID, jobName, submissionDate, s.status, s.stage, interviewDate, offerAmount, startDate, placementFee, rejectionReason, clientFeedback, recruitingOrgID, now, now, recruitingUserID, recruitingUserID)
		if err != nil {
			log.Printf("Warning: Failed to create submission: %v", err)
		} else {
			count++
		}
	}
	return count
}
