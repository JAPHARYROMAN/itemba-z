package reporting

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func renderCSV(id string, reportType ReportType, value any, at time.Time) (ExportArtifact, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	write := func(row ...string) { _ = writer.Write(row) }
	switch v := value.(type) {
	case TrialBalance:
		write("Account code", "Account name", "Type", "Debit minor", "Credit minor")
		for _, l := range v.Lines {
			write(l.Code, l.Name, string(l.Type), number(l.DebitMinor), number(l.CreditMinor))
		}
		write("TOTAL", "", "", number(v.TotalDebitMinor), number(v.TotalCreditMinor))
	case GeneralLedger:
		write("Date", "Journal", "Source type", "Source ID", "Memo", "Debit minor", "Credit minor", "Running balance minor")
		for _, l := range v.Entries {
			write(l.OccurredAt.Format(time.RFC3339), l.JournalID, l.SourceType, l.SourceID, l.Memo, number(l.DebitMinor), number(l.CreditMinor), number(l.RunningBalanceMinor))
		}
	case ProfitAndLoss:
		write("Section", "Account code", "Account name", "Amount minor")
		for _, l := range v.Revenue {
			write("REVENUE", l.Code, l.Name, number(l.AmountMinor))
		}
		for _, l := range v.Expenses {
			write("EXPENSE", l.Code, l.Name, number(l.AmountMinor))
		}
		write("NET PROFIT", "", "", number(v.NetProfitMinor))
	case BalanceSheet:
		write("Section", "Account code", "Account name", "Amount minor")
		for _, l := range v.Assets {
			write("ASSET", l.Code, l.Name, number(l.AmountMinor))
		}
		for _, l := range v.Liabilities {
			write("LIABILITY", l.Code, l.Name, number(l.AmountMinor))
		}
		for _, l := range v.Equity {
			write("EQUITY", l.Code, l.Name, number(l.AmountMinor))
		}
		write("CURRENT EARNINGS", "", "", number(v.CurrentEarningsMinor))
	case CashFlow:
		write("Activity", "Date", "Source type", "Source ID", "Memo", "Amount minor")
		for _, section := range []CashFlowSection{v.Operating, v.Investing, v.Financing, v.Unclassified} {
			for _, l := range section.Lines {
				write(section.Activity, l.OccurredAt.Format(time.RFC3339), l.SourceType, l.SourceID, l.Memo, number(l.AmountMinor))
			}
			write(section.Activity+" NET", "", "", "", "", number(section.NetMinor))
		}
	default:
		return ExportArtifact{}, fmt.Errorf("unsupported report export")
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return ExportArtifact{}, err
	}
	filename := strings.ToLower(strings.ReplaceAll(string(reportType), "_", "-")) + "-" + at.Format("20060102-150405") + ".csv"
	return ExportArtifact{ID: id, ReportType: reportType, Filename: filename, MediaType: "text/csv", ContentBase64: base64.StdEncoding.EncodeToString(buffer.Bytes()), GeneratedAt: at}, nil
}
func number(v int64) string { return strconv.FormatInt(v, 10) }
