Reconciliation Service (Go)
===========================

A small, focused CLI to reconcile internal system transactions against bank statements, designed to address “Example 2: Reconciliation Service (Algorithmic & scaling)” from the Amartha coding exercise.

Alignment to Problem 2
----------------------
- Inputs:
  - System CSV (`-system`)
  - Multiple Bank CSVs (`-bank` repeated)
  - Date range (`-start`, `-end`, format `YYYY-MM-DD`)
- Outputs (summary):
  - `totalProcessed` (system + bank rows within date range)
  - `totalMatched` (by equal IDs)
  - `totalUnmatched` (sum of both sides)
    - `systemMissingInBank` (system rows absent in bank)
    - `bankMissingInSystem` (grouped by bank)
  - `totalAmountDiscrepancyMinor` (sum of absolute amount differences for matched pairs)
  - `matchedWithDiscrepancies` (details when amounts differ)
  - `notes` (e.g., duplicate IDs across banks)
- Assumptions: data from CSV; discrepancies occur only in amounts; IDs are used to match; multiple banks are supported.

Data Model & CSV Formats
------------------------
- System Transactions (required headers):
  - `trxID` (string)
  - `amount` (decimal, no currency symbol)
  - `type` (`DEBIT` | `CREDIT`)
  - `transactionTime` (RFC3339 or `2006-01-02 15:04:05` or `2006-01-02`)
- Bank Statement (required headers):
  - `unique_identifier` (string)
  - `amount` (decimal; negative for debit)
  - `date` (`2006-01-02`)
- Bank name is derived from the file name (without extension), e.g., `bank_bca.csv` → `bank_bca`.
- Amounts are normalized to “minor units” (2 decimals, x100) to avoid floating point issues.
- Sign handling:
  - System: `DEBIT` → negative, `CREDIT` → positive
  - Bank: already signed

Quick Start (Windows PowerShell)
--------------------------------
Build and run:
```powershell
go build -o recon.exe .\cmd\recon
.\recon.exe -system .\testdata\system.csv -bank .\testdata\bank_bca.csv -bank .\testdata\bank_bni.csv -start 2024-01-01 -end 2024-12-31 -json
```
Or run without building:
```powershell
go run .\cmd\recon -system .\testdata\system.csv -bank .\testdata\bank_bca.csv -bank .\testdata\bank_bni.csv -start 2024-01-01 -end 2024-12-31 -json
```
Human-readable output:
```powershell
go run .\cmd\recon -system .\testdata\system.csv -bank .\testdata\bank_bca.csv -bank .\testdata\bank_bni.csv -start 2024-01-01 -end 2024-12-31 -json=false
```
Tip: extract only key fields from JSON:
```powershell
go run .\cmd\recon -system .\testdata\system.csv -bank .\testdata\bank_bca.csv -bank .\testdata\bank_bni.csv -start 2024-01-01 -end 2024-12-31 -json `
| ConvertFrom-Json | Select-Object totalProcessed,totalMatched,totalUnmatched,totalAmountDiscrepancyMinor
```

Datasets
--------
1) Basic fixtures in `testdata/` for quick smoke checks.
2) Full scenario in `testdata/full/` to trigger all outputs at once:
   - Files:
     - `testdata/full/system_full.csv`
     - `testdata/full/bank_bca_full.csv`
     - `testdata/full/bank_bni_full.csv`
   - Run:
```powershell
go run .\cmd\recon `
  -system .\testdata\full\system_full.csv `
  -bank .\testdata\full\bank_bca_full.csv `
  -bank .\testdata\full\bank_bni_full.csv `
  -start 2024-02-01 -end 2024-02-28 -json
```
   - Expected highlights:
     - `totalProcessed` = 15
     - `totalMatched` = 5
     - `totalUnmatched` = 4
       - `systemMissingInBank` contains `S4`
       - `bankMissingInSystem["bank_bca"]` contains `BCA_ONLY1` and `DUP-100`
       - `bankMissingInSystem["bank_bni"]` contains `BNI_ONLY1`
     - `matchedWithDiscrepancies` contains `S3` (amount diff 5.00 → 500 minor)
     - `totalAmountDiscrepancyMinor` = 500
     - `notes` mentions duplicate id `DUP-100` across banks

Testing
-------
Run all tests:
```powershell
go test ./... -v
```
Focused tests:
```powershell
go test ./internal/reconcile -run TestReconcile_SummaryFromFixtures -v
go test ./internal/reconcile -run TestReconcile_SignHandlingAndGrouping -v
go test ./internal/reconcile -run TestReconcile_FullScenario -v
```

Design Notes
------------
- Robust decimal parsing to minor units; avoids external deps.
- Deterministic summaries (sorted) for stable diffs/reviews.
- Duplicate bank IDs are surfaced via `notes`.
- Date filtering at day granularity; times normalized to UTC midnight for date-only comparisons.
- Complexity: O(N) using hash maps over IDs; scales linearly with total rows across files.

Limitations & Extensions
------------------------
- Matches strictly by IDs; no fuzzy matching on amounts or time.
- Assumes two decimal places; adjust parser if a bank uses different precision.
- Easily extendable to:
  - custom matching strategies
  - outputting CSV reports
  - streaming large CSVs
  - parallel reading per bank file

Reference
---------
This solution implements “Example 2: reconciliation service” from the Amartha coding exercise PDF.


