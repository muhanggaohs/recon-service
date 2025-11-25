# Reconciliation Service (Go)

CLI untuk rekonsiliasi transaksi antara data internal dan bank statement.

## Cara Pakai

Bangun dan jalankan:

```bash
go build -o recon ./cmd/recon
./recon -system testdata/system.csv -bank testdata/bank_bca.csv -bank testdata/bank_bni.csv -start 2024-01-01 -end 2024-12-31 -json
```

Flag:
- `-system`: path CSV transaksi sistem
- `-bank`: path CSV bank (bisa banyak kali)
- `-start`: tanggal mulai (YYYY-MM-DD)
- `-end`: tanggal akhir (YYYY-MM-DD)
- `-json`: output JSON (default true). Jika false, output human-readable.

## Format CSV

System Transactions (header wajib):
- `trxID` (string)
- `amount` (decimal, tanpa simbol)
- `type` (`DEBIT`/`CREDIT`)
- `transactionTime` (RFC3339 atau `2006-01-02 15:04:05` atau `2006-01-02`)

Bank Statement (header wajib):
- `unique_identifier` (string)
- `amount` (decimal, negatif untuk debit)
- `date` (`2006-01-02`)

Nama bank diambil dari nama file (tanpa ekstensi), misal `bank_bca.csv` => `bank_bca`.

## Output

Ringkasan JSON berisi:
- `totalProcessed`
- `totalMatched`
- `totalUnmatched`
- `totalAmountDiscrepancyMinor` (penjumlahan selisih absolut dalam minor unit, misal rupiah x100)
- `systemMissingInBank` (detail)
- `bankMissingInSystem` (detail per bank)
- `matchedWithDiscrepancies` (jika ada beda nominal)
- `notes` (misal ada duplikasi ID di bank)

Catatan:
- Nilai uang diproses dalam "minor unit" (2 desimal, x100) agar stabil dan tanpa dependensi eksternal.
- Pencocokan berdasarkan ID yang sama (`trxID` == `unique_identifier`). Perbedaan diasumsikan hanya pada jumlah.


