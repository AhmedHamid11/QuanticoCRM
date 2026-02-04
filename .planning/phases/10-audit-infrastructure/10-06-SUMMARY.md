---
phase: 10-audit-infrastructure
plan: 06
subsystem: ci-security
tags: [gosec, govulncheck, SAST, dependency-scanning, GitHub-Actions, SARIF]

requires:
  - none

provides:
  - security_scanning_workflow: "GitHub Actions workflow for automated security scanning"
  - sast_integration: "gosec static analysis on push to main and PRs"
  - dependency_scanning: "govulncheck for Go dependency vulnerabilities"
  - security_dashboard: "SARIF upload to GitHub Security tab"

affects:
  - ci-pipeline: "All future commits/PRs run security scans"
  - security-posture: "Automated detection of vulnerabilities before production"

tech-stack:
  added:
    - "gosec (GitHub Action securego/gosec@master)"
    - "govulncheck (golang.org/x/vuln/cmd/govulncheck)"
    - "GitHub CodeQL Action (SARIF upload)"
  patterns:
    - "Parallel security jobs (gosec + govulncheck)"
    - "SARIF format for GitHub Security Dashboard integration"
    - "High-severity threshold to avoid false positive fatigue"

key-files:
  created:
    - ".github/workflows/security.yml": "CI security scanning workflow"
  modified: []

decisions:
  - decision: "Use -severity high flag for gosec"
    rationale: "Avoid false positive fatigue while catching critical issues"
    impact: "Medium/low severity findings won't fail build but still appear in SARIF"

  - decision: "Exclude generated code from gosec"
    rationale: "Generated code is not directly maintained by developers"
    impact: "Reduces noise, focuses on actionable findings"

  - decision: "Separate jobs for gosec and govulncheck"
    rationale: "Can run in parallel for faster CI feedback"
    impact: "Faster security scanning (parallel execution)"

  - decision: "Upload SARIF with if: always()"
    rationale: "Upload results even when scan finds issues"
    impact: "GitHub Security tab shows findings even on failed builds"

  - decision: "Skip .gosec config file for now"
    rationale: "No known false positives yet, can add later if needed"
    impact: "Simpler initial setup, can iterate based on real findings"

metrics:
  duration: "0.7 min"
  completed: 2026-02-04
---

# Phase 10 Plan 06: CI Security Scanning Summary

**One-liner:** GitHub Actions workflow with gosec SAST and govulncheck dependency scanning, uploading SARIF to Security Dashboard

## What Was Built

### Security Scanning Workflow (.github/workflows/security.yml)

Created GitHub Actions workflow with two parallel security scanning jobs:

1. **gosec (SAST)**
   - Runs on push to main and pull requests
   - Scans Go code for security vulnerabilities
   - Outputs SARIF format for GitHub Security Dashboard
   - Configured with:
     - `-severity high` - Only fail build on high-severity findings
     - `-exclude-generated` - Skip generated code
     - Path: `./FastCRM/fastcrm/backend/...`
   - Uploads results even if scan finds issues (`if: always()`)

2. **govulncheck (Dependency Scanning)**
   - Runs on push to main and pull requests
   - Checks Go dependencies for known vulnerabilities
   - Working directory: `./FastCRM/fastcrm/backend`
   - Uses official golang.org/x/vuln/cmd/govulncheck tool

### Key Configuration Choices

- **High-severity threshold**: Balances security with developer experience
- **Parallel jobs**: gosec and govulncheck run independently for speed
- **SARIF upload always**: Results appear in GitHub Security tab even on failures
- **No custom gosec config**: Clean initial setup, can tune later based on real findings

## Requirements Satisfied

- **SCAN-01 (SAST)**: gosec runs static analysis on every push/PR
- **SCAN-02 (Dependency Scanning)**: govulncheck monitors Go dependency vulnerabilities
- **Security Dashboard**: SARIF upload enables GitHub Security tab integration
- **Build Gate**: High-severity findings fail the build, blocking merge

## Deviations from Plan

None - plan executed exactly as written.

## Implementation Notes

### gosec Configuration

The workflow uses command-line args rather than a .gosec config file:
- `-fmt sarif` - Output format for GitHub integration
- `-out results.sarif` - File path for SARIF results
- `-exclude-generated` - Skip code generation tools
- `-severity high` - Fail on high-severity only

This approach is simpler than a config file and makes the CI behavior explicit in the workflow file.

### govulncheck Working Directory

Set `working-directory: ./FastCRM/fastcrm/backend` because:
- go.mod is in backend directory
- govulncheck needs module context
- Allows clean `./...` pattern for scanning

### SARIF Upload

The `github/codeql-action/upload-sarif@v3` action:
- Requires `security-events: write` permission (set at workflow level)
- Uses `category: gosec` for distinguishing from other security scans
- Runs with `if: always()` so failures still upload results

## Testing Notes

### Manual Verification Required

1. Push to a branch or create a PR
2. Navigate to Actions tab in GitHub
3. Verify "Security Scanning" workflow runs
4. Check both gosec and govulncheck jobs complete
5. Navigate to Security tab → Code scanning alerts
6. Verify gosec findings appear (if any)

### Expected Behavior

- **Clean code**: Both jobs succeed, no alerts in Security tab
- **Vulnerabilities found**: Build fails, alerts appear in Security tab with file locations and remediation guidance
- **False positives**: Can be addressed later with .gosec config if needed

## Next Phase Readiness

### Blockers

None. Workflow is ready to run on next push to main.

### Concerns

1. **First Run Performance**: Initial gosec/govulncheck run may take longer than subsequent runs (no caching yet)
2. **False Positives**: May need to tune gosec rules based on findings (can add .gosec config file)
3. **Dependency Vulnerabilities**: govulncheck may find existing vulnerabilities that need remediation

### Verification Needs

- Manual: Trigger workflow by pushing to branch
- Verify SARIF upload succeeds (check Security tab)
- Review any findings and determine if legitimate or false positive

## Key Learnings

1. **SARIF Integration**: GitHub's SARIF support provides excellent security finding visibility
2. **Severity Thresholds**: High-severity-only is good default to avoid alert fatigue
3. **Parallel Jobs**: gosec and govulncheck are independent, can run concurrently
4. **Explicit Config**: Putting all gosec flags in workflow args makes CI behavior transparent

## Files Changed

### Created
- `.github/workflows/security.yml` (62 lines) - Security scanning workflow with gosec and govulncheck

### Modified
- None

## Commit Log

- `c9f691f` - feat(10-06): add security scanning to CI pipeline

---

**Status**: Complete ✓
**Requirements**: SCAN-01 (SAST), SCAN-02 (dependency scanning)
**Next**: Manual verification by pushing to branch and checking GitHub Actions + Security tab
