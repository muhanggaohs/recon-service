package reconcile_test

import (
	"path/filepath"
	"testing"
	"time"

	"recon-service/internal/models"
	"recon-service/internal/parser"
	"recon-service/internal/reconcile"
	"recon-service/internal/util"
)

// Note: Comments in English per instruction

func TestReconcile_SummaryFromFixtures(t *testing.T) {
	systemPath := filepath.Join("..", "..", "testdata", "system.csv")
	bcaPath := filepath.Join("..", "..", "testdata", "bank_bca.csv")
	bniPath := filepath.Join("..", "..", "testdata", "bank_bni.csv")

	sysTxns, err := parser.ReadSystemTransactions(systemPath)
	if err != nil {
		t.Fatalf("read system: %v", err)
	}
	bca, err := parser.ReadBankStatements(bcaPath, "bank_bca")
	if err != nil {
		t.Fatalf("read bca: %v", err)
	}
	bni, err := parser.ReadBankStatements(bniPath, "bank_bni")
	if err != nil {
		t.Fatalf("read bni: %v", err)
	}

	start, _ := time.Parse("2006-01-02", "2024-01-01")
	end, _ := time.Parse("2006-01-02", "2024-12-31")
	sys := util.FilterSystemByDate(sysTxns, start, end)
	banks := util.FilterBanksByDate([]*parser.BankFile{bca, bni}, start, end)

	sum := reconcile.Reconcile(sys, banks)

	if sum.TotalProcessed != 11 {
		t.Fatalf("TotalProcessed got=%d want=%d", sum.TotalProcessed, 11)
	}
	if sum.TotalMatched != 5 {
		t.Fatalf("TotalMatched got=%d want=%d", sum.TotalMatched, 5)
	}
	if sum.TotalUnmatched != 1 {
		t.Fatalf("TotalUnmatched got=%d want=%d", sum.TotalUnmatched, 1)
	}
	if len(sum.SystemMissingInBank) != 0 {
		t.Fatalf("SystemMissingInBank len got=%d want=%d", len(sum.SystemMissingInBank), 0)
	}
	if len(sum.BankMissingInSystem["bank_bca"]) != 1 {
		t.Fatalf("BankMissingInSystem[bank_bca] len got=%d want=%d", len(sum.BankMissingInSystem["bank_bca"]), 1)
	}
	if sum.TotalAmountDiscrepancy != 50 {
		t.Fatalf("TotalAmountDiscrepancyMinor got=%d want=%d", sum.TotalAmountDiscrepancy, 50)
	}
}

func TestReconcile_SignHandlingAndGrouping(t *testing.T) {
	// Construct minimal in-memory data
	sys := []models.SystemTransaction{
		{TrxID: "SAME-1", AmountMinor: 10000, Type: models.TypeCredit, TransactionTime: mustDate("2024-01-01")},
		{TrxID: "SAME-2", AmountMinor: 25000, Type: models.TypeDebit, TransactionTime: mustDate("2024-01-01")},
	}
	// Banks: one matches credit positive, one matches debit negative
	b1 := &parser.BankFile{
		BankName: "bank_a",
		Rows: []models.BankStatement{
			{UniqueIdentifier: "SAME-1", AmountMinor: 10000, Date: mustDate("2024-01-01"), BankName: "bank_a"},
			{UniqueIdentifier: "ONLY-A", AmountMinor: 12345, Date: mustDate("2024-01-01"), BankName: "bank_a"},
		},
	}
	b2 := &parser.BankFile{
		BankName: "bank_b",
		Rows: []models.BankStatement{
			{UniqueIdentifier: "SAME-2", AmountMinor: -25000, Date: mustDate("2024-01-01"), BankName: "bank_b"},
			{UniqueIdentifier: "ONLY-B", AmountMinor: -1, Date: mustDate("2024-01-01"), BankName: "bank_b"},
		},
	}

	sum := reconcile.Reconcile(sys, []*parser.BankFile{b1, b2})

	if sum.TotalMatched != 2 {
		t.Fatalf("TotalMatched got=%d want=%d", sum.TotalMatched, 2)
	}
	if sum.TotalAmountDiscrepancy != 0 {
		t.Fatalf("TotalAmountDiscrepancy got=%d want=%d", sum.TotalAmountDiscrepancy, 0)
	}
	// two missing (one per bank)
	if len(sum.BankMissingInSystem["bank_a"]) != 1 || len(sum.BankMissingInSystem["bank_b"]) != 1 {
		t.Fatalf("BankMissingInSystem grouping unexpected: %+v", sum.BankMissingInSystem)
	}
	// none missing from system point of view (since both SAME-1/2 found)
	if len(sum.SystemMissingInBank) != 0 {
		t.Fatalf("SystemMissingInBank got=%d want=%d", len(sum.SystemMissingInBank), 0)
	}
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestReconcile_FullScenario(t *testing.T) {
	systemPath := filepath.Join("..", "..", "testdata", "full", "system_full.csv")
	bcaPath := filepath.Join("..", "..", "testdata", "full", "bank_bca_full.csv")
	bniPath := filepath.Join("..", "..", "testdata", "full", "bank_bni_full.csv")

	sysTxns, err := parser.ReadSystemTransactions(systemPath)
	if err != nil {
		t.Fatalf("read system: %v", err)
	}
	bca, err := parser.ReadBankStatements(bcaPath, "bank_bca")
	if err != nil {
		t.Fatalf("read bca: %v", err)
	}
	bni, err := parser.ReadBankStatements(bniPath, "bank_bni")
	if err != nil {
		t.Fatalf("read bni: %v", err)
	}

	start, _ := time.Parse("2006-01-02", "2024-02-01")
	end, _ := time.Parse("2006-01-02", "2024-02-28")
	sys := util.FilterSystemByDate(sysTxns, start, end)
	banks := util.FilterBanksByDate([]*parser.BankFile{bca, bni}, start, end)

	sum := reconcile.Reconcile(sys, banks)

	// Expectations:
	// System rows: 6
	// BCA rows: 6; BNI rows: 3 -> totalProcessed = 15
	if sum.TotalProcessed != 15 {
		t.Fatalf("TotalProcessed got=%d want=%d", sum.TotalProcessed, 15)
	}
	// Matched: S1, S2, S3, S5, S6 => 5
	if sum.TotalMatched != 5 {
		t.Fatalf("TotalMatched got=%d want=%d", sum.TotalMatched, 5)
	}
	// System missing: S4 => 1
	if len(sum.SystemMissingInBank) != 1 {
		t.Fatalf("SystemMissingInBank len got=%d want=%d", len(sum.SystemMissingInBank), 1)
	}
	// Bank missing: bank_bca has BCA_ONLY1 and DUP-100; bank_bni has BNI_ONLY1
	if len(sum.BankMissingInSystem["bank_bca"]) != 2 {
		t.Fatalf("BankMissingInSystem[bank_bca] len got=%d want=%d", len(sum.BankMissingInSystem["bank_bca"]), 2)
	}
	if len(sum.BankMissingInSystem["bank_bni"]) != 1 {
		t.Fatalf("BankMissingInSystem[bank_bni] len got=%d want=%d", len(sum.BankMissingInSystem["bank_bni"]), 1)
	}
	// Total unmatched = 1 + 2 + 1 = 4
	if sum.TotalUnmatched != 4 {
		t.Fatalf("TotalUnmatched got=%d want=%d", sum.TotalUnmatched, 4)
	}
	// Discrepancy: S3 diff 5.00 => 500 minor
	if sum.TotalAmountDiscrepancy != 500 {
		t.Fatalf("TotalAmountDiscrepancy got=%d want=%d", sum.TotalAmountDiscrepancy, 500)
	}
	// There should be at least one discrepancy detail (S3)
	if len(sum.MatchedWithDiscrepancies) == 0 {
		t.Fatalf("MatchedWithDiscrepancies should not be empty")
	}
	// Notes should contain duplicate id info (DUP-100)
	if len(sum.Notes) == 0 {
		t.Fatalf("Notes should not be empty (expect duplicates)")
	}
}
