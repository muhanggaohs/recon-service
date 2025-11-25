package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"recon-service/internal/models"
)

// Note: Comments in English per instruction

type BankFile struct {
	BankName string
	Rows     []models.BankStatement
}

// ReadSystemTransactions reads CSV with headers:
// trxID,amount,type,transactionTime
// amount: decimal string, parsed into minor units (x100)
// transactionTime: RFC3339 or "2006-01-02 15:04:05" or "2006-01-02"
func ReadSystemTransactions(path string) ([]models.SystemTransaction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := toIndex(headers)
	required := []string{"trxID", "amount", "type", "transactionTime"}
	for _, k := range required {
		if _, ok := col[k]; !ok {
			return nil, fmt.Errorf("missing column: %s", k)
		}
	}

	var out []models.SystemTransaction
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		trxID := rec[col["trxID"]]
		amountStr := rec[col["amount"]]
		typStr := strings.ToUpper(strings.TrimSpace(rec[col["type"]]))
		timeStr := rec[col["transactionTime"]]

		amountMinor, err := parseDecimalToMinor(amountStr)
		if err != nil {
			return nil, fmt.Errorf("row trxID=%s amount parse: %w", trxID, err)
		}
		tt, err := parseTimeFlexible(timeStr)
		if err != nil {
			return nil, fmt.Errorf("row trxID=%s time parse: %w", trxID, err)
		}
		typ := models.TransactionType(typStr)
		switch typ {
		case models.TypeDebit, models.TypeCredit:
		default:
			return nil, fmt.Errorf("row trxID=%s invalid type: %s", trxID, typStr)
		}
		out = append(out, models.SystemTransaction{
			TrxID:           trxID,
			AmountMinor:     amountMinor,
			Type:            typ,
			TransactionTime: tt,
		})
	}
	return out, nil
}

// ReadBankStatements reads bank CSV with headers:
// unique_identifier,amount,date
// amount may be negative for debit
// date: "2006-01-02"
func ReadBankStatements(path string, bankName string) (*BankFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := toIndex(headers)
	required := []string{"unique_identifier", "amount", "date"}
	for _, k := range required {
		if _, ok := col[k]; !ok {
			return nil, fmt.Errorf("missing column: %s", k)
		}
	}
	var rows []models.BankStatement
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		id := rec[col["unique_identifier"]]
		amountStr := rec[col["amount"]]
		dateStr := rec[col["date"]]
		amountMinor, err := parseDecimalToMinor(amountStr)
		if err != nil {
			return nil, fmt.Errorf("row uid=%s amount parse: %w", id, err)
		}
		// date-only normalized to midnight
		dt, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("row uid=%s date parse: %w", id, err)
		}
		dt = time.Date(dt.Year(), dt.Month(), dt.Day(), 0, 0, 0, 0, time.UTC)
		rows = append(rows, models.BankStatement{
			UniqueIdentifier: id,
			AmountMinor:      amountMinor,
			Date:             dt,
			BankName:         bankName,
		})
	}
	return &BankFile{
		BankName: bankName,
		Rows:     rows,
	}, nil
}

func toIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

// parseDecimalToMinor converts "1234.56" -> 123456 minor units (2 decimals).
// It tolerates comma or dot as decimal separator, and strips thousand separators.
func parseDecimalToMinor(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	// Normalize decimal separator: if both '.' and ',' exist, assume ',' is thousands and '.' is decimal
	// If only ',' exists, treat it as decimal
	if strings.Contains(s, ".") && strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", "")
	} else if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	// Remove any stray spaces/quotes
	s = strings.ReplaceAll(s, " ", "")
	s = strings.Trim(s, "\"")

	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	// Remove thousand separators if any left
	intPart = strings.ReplaceAll(intPart, ",", "")

	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > 2 {
		fracPart = fracPart[:2]
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}
	full := intPart + fracPart
	if full == "" {
		full = "0"
	}
	v, err := strconv.ParseInt(full, 10, 64)
	if err != nil {
		return 0, err
	}
	if neg {
		v = -v
	}
	return v, nil
}

func parseTimeFlexible(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	var lastErr error
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			// Normalize to UTC
			if f == "2006-01-02" {
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			}
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("time parse failed: %w", lastErr)
}


