package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"recon-service/internal/parser"
	"recon-service/internal/reconcile"
	"recon-service/internal/util"
)

// Note: Comments in English per instruction
func main() {
	var systemCSV string
	var bankCSVPaths multiString
	var startDateStr string
	var endDateStr string
	var outputJSON bool

	flag.StringVar(&systemCSV, "system", "", "Path to system transactions CSV")
	flag.Var(&bankCSVPaths, "bank", "Path to bank statement CSV (can be specified multiple times)")
	flag.StringVar(&startDateStr, "start", "", "Start date (YYYY-MM-DD)")
	flag.StringVar(&endDateStr, "end", "", "End date (YYYY-MM-DD)")
	flag.BoolVar(&outputJSON, "json", true, "Output JSON summary")
	flag.Parse()

	if systemCSV == "" || len(bankCSVPaths) == 0 || startDateStr == "" || endDateStr == "" {
		flag.Usage()
		os.Exit(2)
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		log.Fatalf("invalid start date: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		log.Fatalf("invalid end date: %v", err)
	}
	if endDate.Before(startDate) {
		log.Fatalf("end date must be on/after start date")
	}

	sysTxns, err := parser.ReadSystemTransactions(systemCSV)
	if err != nil {
		log.Fatalf("read system csv failed: %v", err)
	}

	var bankAll []*parser.BankFile
	for _, p := range bankCSVPaths {
		name := bankNameFromPath(p)
		records, err := parser.ReadBankStatements(p, name)
		if err != nil {
			log.Fatalf("read bank csv failed (%s): %v", p, err)
		}
		bankAll = append(bankAll, records)
	}

	// Normalize and filter by date range
	sysFiltered := util.FilterSystemByDate(sysTxns, startDate, endDate)
	bankFiltered := util.FilterBanksByDate(bankAll, startDate, endDate)

	res := reconcile.Reconcile(sysFiltered, bankFiltered)

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			log.Fatalf("encode json failed: %v", err)
		}
	} else {
		fmt.Println(reconcile.HumanSummary(res))
	}
}

type multiString []string

func (m *multiString) String() string {
	return strings.Join(*m, ",")
}

func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func bankNameFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	if base == "" {
		return "bank"
	}
	return base
}


