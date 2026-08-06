package reporting

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

type reportTable struct {
	Title, Currency, CurrentLabel, ComparisonLabel string
	Headers                                        []string
	Rows                                           [][]any
}

func renderReport(id string, pack ReportPack, format ExportFormat, at time.Time) (ExportArtifact, error) {
	table, err := tabulate(pack)
	if err != nil {
		return ExportArtifact{}, err
	}
	var content []byte
	media, extension := "", ""
	switch format {
	case ExportCSV:
		content, err = renderTableCSV(table)
		media, extension = "text/csv", "csv"
	case ExportPDF:
		content, err = renderTablePDF(table, at)
		media, extension = "application/pdf", "pdf"
	case ExportXLSX:
		content, err = renderTableXLSX(table, at)
		media, extension = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
	default:
		return ExportArtifact{}, ErrInvalidQuery
	}
	if err != nil {
		return ExportArtifact{}, err
	}
	sum := sha256.Sum256(content)
	name := strings.ToLower(strings.ReplaceAll(string(pack.ReportType), "_", "-")) + "-" + at.Format("20060102-150405") + "." + extension
	return ExportArtifact{ID: id, ReportType: pack.ReportType, Format: format, Filename: name, MediaType: media, SHA256: hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(content), GeneratedAt: at}, nil
}

func tabulate(pack ReportPack) (reportTable, error) {
	t := reportTable{Title: strings.ReplaceAll(string(pack.ReportType), "_", " "), CurrentLabel: pack.CurrentLabel, ComparisonLabel: pack.ComparisonLabel}
	comparisonHeaders := func() []string {
		if pack.Comparison == nil {
			return []string{"Current (minor)"}
		}
		return []string{"Current (minor)", "Comparison (minor)", "Variance (minor)"}
	}
	appendAmounts := func(row []any, current, prior int64) []any {
		row = append(row, current)
		if pack.Comparison != nil {
			row = append(row, prior, current-prior)
		}
		return row
	}
	statementRows := func(current []StatementLine, prior []StatementLine, section string) {
		priorByCode := map[string]int64{}
		for _, line := range prior {
			priorByCode[line.Code] = line.AmountMinor
		}
		seen := map[string]bool{}
		for _, line := range current {
			t.Rows = append(t.Rows, appendAmounts([]any{section, line.Code, line.Name}, line.AmountMinor, priorByCode[line.Code]))
			seen[line.Code] = true
		}
		for _, line := range prior {
			if !seen[line.Code] {
				t.Rows = append(t.Rows, appendAmounts([]any{section, line.Code, line.Name}, 0, line.AmountMinor))
			}
		}
	}
	switch current := pack.Current.(type) {
	case ProfitAndLoss:
		t.Currency = current.Currency
		t.Headers = append([]string{"Section", "Account code", "Account name"}, comparisonHeaders()...)
		prior := ProfitAndLoss{}
		if pack.Comparison != nil {
			var ok bool
			prior, ok = pack.Comparison.(ProfitAndLoss)
			if !ok {
				return t, ErrInvalidQuery
			}
		}
		statementRows(current.Revenue, prior.Revenue, "REVENUE")
		statementRows(current.Expenses, prior.Expenses, "EXPENSE")
		t.Rows = append(t.Rows, appendAmounts([]any{"TOTAL", "", "Net profit"}, current.NetProfitMinor, prior.NetProfitMinor))
	case BalanceSheet:
		t.Currency = current.Currency
		t.Headers = append([]string{"Section", "Account code", "Account name"}, comparisonHeaders()...)
		prior := BalanceSheet{}
		if pack.Comparison != nil {
			var ok bool
			prior, ok = pack.Comparison.(BalanceSheet)
			if !ok {
				return t, ErrInvalidQuery
			}
		}
		statementRows(current.Assets, prior.Assets, "ASSET")
		statementRows(current.Liabilities, prior.Liabilities, "LIABILITY")
		statementRows(current.Equity, prior.Equity, "EQUITY")
		t.Rows = append(t.Rows, appendAmounts([]any{"TOTAL", "", "Total assets"}, current.TotalAssetsMinor, prior.TotalAssetsMinor), appendAmounts([]any{"TOTAL", "", "Liabilities and equity"}, current.TotalLiabilitiesMinor+current.TotalEquityMinor, prior.TotalLiabilitiesMinor+prior.TotalEquityMinor))
	case CashFlow:
		t.Currency = current.Currency
		t.Headers = append([]string{"Activity", "Metric", "Description"}, comparisonHeaders()...)
		prior := CashFlow{}
		if pack.Comparison != nil {
			var ok bool
			prior, ok = pack.Comparison.(CashFlow)
			if !ok {
				return t, ErrInvalidQuery
			}
		}
		for _, v := range []struct {
			name           string
			current, prior int64
		}{{"Opening cash", current.OpeningCashMinor, prior.OpeningCashMinor}, {"Operating net", current.Operating.NetMinor, prior.Operating.NetMinor}, {"Investing net", current.Investing.NetMinor, prior.Investing.NetMinor}, {"Financing net", current.Financing.NetMinor, prior.Financing.NetMinor}, {"Unclassified net", current.Unclassified.NetMinor, prior.Unclassified.NetMinor}, {"Closing cash", current.ClosingCashMinor, prior.ClosingCashMinor}} {
			t.Rows = append(t.Rows, appendAmounts([]any{"CASH FLOW", "", v.name}, v.current, v.prior))
		}
	case TrialBalance:
		if pack.Comparison != nil {
			return t, ErrInvalidQuery
		}
		t.Currency = current.Currency
		t.Headers = []string{"Account code", "Account name", "Type", "Debit (minor)", "Credit (minor)"}
		for _, l := range current.Lines {
			t.Rows = append(t.Rows, []any{l.Code, l.Name, string(l.Type), l.DebitMinor, l.CreditMinor})
		}
		t.Rows = append(t.Rows, []any{"TOTAL", "", "", current.TotalDebitMinor, current.TotalCreditMinor})
	case GeneralLedger:
		if pack.Comparison != nil {
			return t, ErrInvalidQuery
		}
		t.Currency = current.Currency
		t.Headers = []string{"Date", "Source type", "Source ID", "Memo", "Debit (minor)", "Credit (minor)", "Balance (minor)"}
		for _, l := range current.Entries {
			t.Rows = append(t.Rows, []any{l.OccurredAt.Format("2006-01-02"), l.SourceType, l.SourceID, l.Memo, l.DebitMinor, l.CreditMinor, l.RunningBalanceMinor})
		}
	default:
		return t, ErrInvalidQuery
	}
	sort.SliceStable(t.Rows, func(i, j int) bool {
		return fmt.Sprint(t.Rows[i][0], t.Rows[i][1]) < fmt.Sprint(t.Rows[j][0], t.Rows[j][1])
	})
	return t, nil
}

func renderTableCSV(t reportTable) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string{t.Title, t.Currency, t.CurrentLabel, t.ComparisonLabel})
	_ = w.Write(t.Headers)
	for _, row := range t.Rows {
		values := make([]string, len(row))
		for i, v := range row {
			values[i] = safeCSVCell(fmt.Sprint(v))
		}
		_ = w.Write(values)
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func safeCSVCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func renderTablePDF(t reportTable, at time.Time) ([]byte, error) {
	p := fpdf.New("L", "mm", "A4", "")
	p.AddUTF8FontFromBytes("Itemba", "", goregular.TTF)
	p.AddUTF8FontFromBytes("Itemba", "B", gobold.TTF)
	p.SetTitle(t.Title, false)
	p.SetCreator("ITEMBA-Z", false)
	p.SetCreationDate(at)
	p.SetMargins(12, 12, 12)
	p.SetAutoPageBreak(true, 14)
	p.AliasNbPages("")
	p.SetFooterFunc(func() {
		p.SetY(-10)
		p.SetFont("Itemba", "", 8)
		p.SetTextColor(90, 100, 100)
		p.CellFormat(0, 5, fmt.Sprintf("ITEMBA-Z | Generated %s | Page %d/{nb}", at.Format(time.RFC3339), p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	p.SetFillColor(11, 43, 38)
	p.Rect(0, 0, 297, 25, "F")
	p.SetTextColor(255, 255, 255)
	p.SetFont("Itemba", "B", 18)
	p.CellFormat(0, 9, t.Title, "", 1, "L", false, 0, "")
	p.SetFont("Itemba", "", 9)
	p.CellFormat(0, 6, fmt.Sprintf("Currency: %s | Current: %s | Comparison: %s", t.Currency, t.CurrentLabel, blankDash(t.ComparisonLabel)), "", 1, "L", false, 0, "")
	p.Ln(5)
	width := 273 / float64(len(t.Headers))
	widths := make([]float64, len(t.Headers))
	for i := range widths {
		widths[i] = width
	}
	if len(widths) > 2 {
		widths[0] = 28
		widths[1] = 30
		widths[2] = 273 - float64(len(widths)-3)*widths[3] - 58
	}
	row := func(values []string, header bool) {
		if header {
			p.SetFillColor(25, 102, 89)
			p.SetTextColor(255, 255, 255)
			p.SetFont("Itemba", "B", 8)
		} else {
			p.SetFillColor(246, 249, 248)
			p.SetTextColor(25, 35, 35)
			p.SetFont("Itemba", "", 8)
		}
		for i, v := range values {
			align := "L"
			if i >= len(values)-3 {
				align = "R"
			}
			p.CellFormat(widths[i], 6, truncate(v, 42), "B", 0, align, true, 0, "")
		}
		p.Ln(-1)
	}
	row(t.Headers, true)
	for _, values := range t.Rows {
		if p.GetY() > 185 {
			p.AddPage()
			row(t.Headers, true)
		}
		cells := make([]string, len(values))
		for i, v := range values {
			cells[i] = display(v)
		}
		row(cells, false)
	}
	var b bytes.Buffer
	if err := p.Output(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func renderTableXLSX(t reportTable, at time.Time) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Financial Report"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	f.SetCellValue(sheet, "A1", t.Title)
	f.MergeCell(sheet, "A1", column(len(t.Headers))+"1")
	f.SetCellValue(sheet, "A2", "Currency")
	f.SetCellValue(sheet, "B2", t.Currency)
	f.SetCellValue(sheet, "C2", "Current")
	f.SetCellValue(sheet, "D2", t.CurrentLabel)
	if t.ComparisonLabel != "" {
		f.SetCellValue(sheet, "E2", "Comparison")
		f.SetCellValue(sheet, "F2", t.ComparisonLabel)
	}
	f.SetCellValue(sheet, "A3", "Generated")
	f.SetCellValue(sheet, "B3", at)
	for i, h := range t.Headers {
		f.SetCellValue(sheet, column(i+1)+"5", h)
	}
	for r, row := range t.Rows {
		for c, v := range row {
			if text, ok := v.(string); ok && text == "" {
				continue
			}
			f.SetCellValue(sheet, column(c+1)+strconv.Itoa(r+6), v)
		}
	}
	title, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 16}, Fill: excelize.Fill{Type: "pattern", Color: []string{"0B2B26"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "center"}})
	header, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"196659"}, Pattern: 1}, Alignment: &excelize.Alignment{WrapText: true}, Border: []excelize.Border{{Type: "bottom", Color: "D7E4E1", Style: 1}}})
	money, _ := f.NewStyle(&excelize.Style{NumFmt: 3})
	date, _ := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr("yyyy-mm-dd hh:mm")})
	f.SetCellStyle(sheet, "A1", column(len(t.Headers))+"1", title)
	f.SetRowHeight(sheet, 1, 28)
	f.SetCellStyle(sheet, "A5", column(len(t.Headers))+"5", header)
	f.SetCellStyle(sheet, "B3", "B3", date)
	if len(t.Rows) > 0 {
		start := max(1, len(t.Headers)-2)
		f.SetCellStyle(sheet, column(start)+"6", column(len(t.Headers))+strconv.Itoa(len(t.Rows)+5), money)
	}
	f.SetColWidth(sheet, "A", column(len(t.Headers)), 18)
	if len(t.Headers) >= 3 {
		f.SetColWidth(sheet, "C", "C", 34)
	}
	if len(t.Headers) >= 4 {
		f.SetColWidth(sheet, "D", "D", 26)
	}
	if len(t.Headers) >= 6 {
		f.SetColWidth(sheet, "F", "F", 26)
	}
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 5, TopLeftCell: "A6", ActivePane: "bottomLeft"})
	f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: boolPtr(false)})
	f.SetPageLayout(sheet, &excelize.PageLayoutOptions{Orientation: strPtr("landscape"), Size: intPtr(9), FitToWidth: intPtr(1), FitToHeight: intPtr(0)})
	f.SetHeaderFooter(sheet, &excelize.HeaderFooterOptions{OddFooter: "&CITEMBA-Z | Page &P of &N"})
	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func column(n int) string { v, _ := excelize.ColumnNumberToName(n); return v }
func display(v any) string {
	switch n := v.(type) {
	case int64:
		return fmt.Sprintf("%d", n)
	case int:
		return fmt.Sprintf("%d", n)
	default:
		return fmt.Sprint(v)
	}
}
func blankDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
func truncate(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return v
	}
	return string(r[:n-1]) + "..."
}
func strPtr(v string) *string { return &v }
func boolPtr(v bool) *bool    { return &v }
func intPtr(v int) *int       { return &v }
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
