package util

import (
	"time"

	"recon-service/internal/models"
	"recon-service/internal/parser"
)

// Note: Comments in English per instruction

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func betweenDays(t time.Time, start, end time.Time) bool {
	td := dateOnly(t)
	sd := dateOnly(start)
	ed := dateOnly(end)
	return (td.Equal(sd) || td.After(sd)) && (td.Equal(ed) || td.Before(ed))
}

func FilterSystemByDate(in []models.SystemTransaction, start, end time.Time) []models.SystemTransaction {
	out := make([]models.SystemTransaction, 0, len(in))
	for _, x := range in {
		if betweenDays(x.TransactionTime, start, end) {
			out = append(out, x)
		}
	}
	return out
}

func FilterBanksByDate(files []*parser.BankFile, start, end time.Time) []*parser.BankFile {
	var out []*parser.BankFile
	for _, bf := range files {
		var rows []models.BankStatement
		for _, r := range bf.Rows {
			if betweenDays(r.Date, start, end) {
				rows = append(rows, r)
			}
		}
		out = append(out, &parser.BankFile{
			BankName: bf.BankName,
			Rows:     rows,
		})
	}
	return out
}


