package reconcile

import (
	"fmt"
	"sort"
	"strings"

	"recon-service/internal/models"
	"recon-service/internal/parser"
)

// Note: Comments in English per instruction

type UnmatchedSystem struct {
	TrxID       string `json:"trxID"`
	AmountMinor int64  `json:"amountMinor"`
	Type        string `json:"type"`
}

type UnmatchedBank struct {
	UniqueIdentifier string `json:"unique_identifier"`
	AmountMinor      int64  `json:"amountMinor"`
	BankName         string `json:"bank"`
}

type MatchedDiff struct {
	ID                string `json:"id"`
	SystemAmountMinor int64  `json:"systemAmountMinor"`
	BankAmountMinor   int64  `json:"bankAmountMinor"`
	AbsDiffMinor      int64  `json:"absDiffMinor"`
	BankName          string `json:"bank"`
}

type Summary struct {
	TotalProcessed           int                          `json:"totalProcessed"`
	TotalMatched             int                          `json:"totalMatched"`
	TotalUnmatched           int                          `json:"totalUnmatched"`
	TotalAmountDiscrepancy   int64                        `json:"totalAmountDiscrepancyMinor"`
	SystemMissingInBank      []UnmatchedSystem            `json:"systemMissingInBank"`
	BankMissingInSystem      map[string][]UnmatchedBank   `json:"bankMissingInSystem"`
	MatchedWithDiscrepancies []MatchedDiff                `json:"matchedWithDiscrepancies"`
	Notes                    []string                     `json:"notes,omitempty"`
}

func Reconcile(systemTxns []models.SystemTransaction, bankFiles []*parser.BankFile) Summary {
	// system map by id
	sysByID := map[string]models.SystemTransaction{}
	for _, s := range systemTxns {
		sysByID[s.TrxID] = s
	}
	// bank map by id (store first occurrence + bank name)
	type bankEntry struct {
		row models.BankStatement
	}
	banked := map[string]bankEntry{}
	// Track duplicates in bank ids for transparency
	duplicateBankIDs := map[string][]string{} // id -> banks
	for _, bf := range bankFiles {
		for _, r := range bf.Rows {
			if exist, ok := banked[r.UniqueIdentifier]; ok {
				duplicateBankIDs[r.UniqueIdentifier] = appendUnique(duplicateBankIDs[r.UniqueIdentifier], exist.row.BankName, r.BankName)
				// Keep the first one deterministically (existing)
				continue
			}
			banked[r.UniqueIdentifier] = bankEntry{row: r}
		}
	}

	totalProcessed := len(systemTxns)
	for _, bf := range bankFiles {
		totalProcessed += len(bf.Rows)
	}

	var totalMatched int
	var totalAmountDiscrepancy int64
	var sysMissing []UnmatchedSystem
	bankMissingGrouped := map[string][]UnmatchedBank{}
	var matchedDiffs []MatchedDiff

	// Matched and system-missing
	for id, s := range sysByID {
		if be, ok := banked[id]; ok {
			totalMatched++
			sysSigned, _ := s.Type.SignedAmount(s.AmountMinor)
			bankSigned := be.row.AmountMinor
			diff := abs64(sysSigned - bankSigned)
			if diff != 0 {
				totalAmountDiscrepancy += abs64(diff)
				matchedDiffs = append(matchedDiffs, MatchedDiff{
					ID:                id,
					SystemAmountMinor: sysSigned,
					BankAmountMinor:   bankSigned,
					AbsDiffMinor:      abs64(diff),
					BankName:          be.row.BankName,
				})
			}
		} else {
			sysMissing = append(sysMissing, UnmatchedSystem{
				TrxID:       id,
				AmountMinor: s.AmountMinor,
				Type:        string(s.Type),
			})
		}
	}

	// Bank-missing
	for id, be := range banked {
		if _, ok := sysByID[id]; !ok {
			bankMissingGrouped[be.row.BankName] = append(bankMissingGrouped[be.row.BankName], UnmatchedBank{
				UniqueIdentifier: id,
				AmountMinor:      be.row.AmountMinor,
				BankName:         be.row.BankName,
			})
		}
	}

	// Deterministic ordering
	sort.Slice(sysMissing, func(i, j int) bool { return sysMissing[i].TrxID < sysMissing[j].TrxID })
	for bank := range bankMissingGrouped {
		sort.Slice(bankMissingGrouped[bank], func(i, j int) bool {
			return bankMissingGrouped[bank][i].UniqueIdentifier < bankMissingGrouped[bank][j].UniqueIdentifier
		})
	}
	sort.Slice(matchedDiffs, func(i, j int) bool { return matchedDiffs[i].ID < matchedDiffs[j].ID })

	var notes []string
	if len(duplicateBankIDs) > 0 {
		notes = append(notes, formatDuplicateNotes(duplicateBankIDs))
	}

	totalUnmatched := len(sysMissing)
	for _, v := range bankMissingGrouped {
		totalUnmatched += len(v)
	}

	return Summary{
		TotalProcessed:           totalProcessed,
		TotalMatched:             totalMatched,
		TotalUnmatched:           totalUnmatched,
		TotalAmountDiscrepancy:   totalAmountDiscrepancy,
		SystemMissingInBank:      sysMissing,
		BankMissingInSystem:      bankMissingGrouped,
		MatchedWithDiscrepancies: matchedDiffs,
		Notes:                    notes,
	}
}

func HumanSummary(s Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Total processed: %d\n", s.TotalProcessed)
	fmt.Fprintf(&b, "Total matched: %d\n", s.TotalMatched)
	fmt.Fprintf(&b, "Total unmatched: %d\n", s.TotalUnmatched)
	fmt.Fprintf(&b, "Total amount discrepancy (minor): %d\n", s.TotalAmountDiscrepancy)
	if len(s.MatchedWithDiscrepancies) > 0 {
		fmt.Fprintf(&b, "\nMatched with amount differences:\n")
		for _, d := range s.MatchedWithDiscrepancies {
			fmt.Fprintf(&b, "- %s (bank=%s): system=%d bank=%d diff=%d\n",
				d.ID, d.BankName, d.SystemAmountMinor, d.BankAmountMinor, d.AbsDiffMinor)
		}
	}
	if len(s.SystemMissingInBank) > 0 {
		fmt.Fprintf(&b, "\nSystem missing in bank:\n")
		for _, u := range s.SystemMissingInBank {
			fmt.Fprintf(&b, "- %s (%s) amountMinor=%d\n", u.TrxID, u.Type, u.AmountMinor)
		}
	}
	if len(s.BankMissingInSystem) > 0 {
		fmt.Fprintf(&b, "\nBank missing in system:\n")
		// stable order of banks
		banks := make([]string, 0, len(s.BankMissingInSystem))
		for k := range s.BankMissingInSystem {
			banks = append(banks, k)
		}
		sort.Strings(banks)
		for _, bank := range banks {
			fmt.Fprintf(&b, "  [%s]\n", bank)
			for _, u := range s.BankMissingInSystem[bank] {
				fmt.Fprintf(&b, "  - %s amountMinor=%d\n", u.UniqueIdentifier, u.AmountMinor)
			}
		}
	}
	if len(s.Notes) > 0 {
		fmt.Fprintf(&b, "\nNotes:\n")
		for _, n := range s.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	return b.String()
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func appendUnique(dst []string, vals ...string) []string {
	set := map[string]struct{}{}
	for _, d := range dst {
		set[d] = struct{}{}
	}
	for _, v := range vals {
		if _, ok := set[v]; !ok {
			dst = append(dst, v)
			set[v] = struct{}{}
		}
	}
	return dst
}

func formatDuplicateNotes(dups map[string][]string) string {
	// Produce compact note string
	keys := make([]string, 0, len(dups))
	for k := range dups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, id := range keys {
		parts = append(parts, fmt.Sprintf("%s in banks=%v", id, dups[id]))
	}
	return "duplicate bank IDs detected: " + strings.Join(parts, "; ")
}


