package workforce

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
)

type ExportRow struct {
	EmployeeNumber, FullName, Currency, PaymentDate, PayrollReference string
	GrossMinor, OtherDeductionsMinor, LoanDeductionMinor, NetMinor    int64
}
type exportConfiguration struct {
	Format         ExportFormat `json:"format"`
	Delimiter      string       `json:"delimiter"`
	IncludeHeader  bool         `json:"include_header"`
	Columns        []string     `json:"columns"`
	FileNamePrefix string       `json:"file_name_prefix"`
}

var allowedColumns = map[string]bool{"employee_number": true, "full_name": true, "gross_minor": true, "other_deductions_minor": true, "loan_deduction_minor": true, "total_deductions_minor": true, "net_minor": true, "currency": true, "payment_date": true, "payroll_reference": true}

func BuildPayrollCSV(raw []byte, expectedFormat ExportFormat, rows []ExportRow) (string, string, error) {
	var c exportConfiguration
	if json.Unmarshal(raw, &c) != nil || c.Format != expectedFormat || len(c.Columns) == 0 || len(c.Columns) > 20 {
		return "", "", ErrConfiguration
	}
	delimiter := ','
	if c.Delimiter != "" {
		r := []rune(c.Delimiter)
		if len(r) != 1 || (r[0] != ',' && r[0] != ';' && r[0] != '\t') {
			return "", "", ErrConfiguration
		}
		delimiter = r[0]
	}
	for _, column := range c.Columns {
		if !allowedColumns[column] {
			return "", "", ErrConfiguration
		}
	}
	prefix := strings.TrimSpace(c.FileNamePrefix)
	if prefix == "" || len(prefix) > 80 || fileComponent(prefix) != prefix {
		return "", "", ErrConfiguration
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].EmployeeNumber < rows[j].EmployeeNumber })
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = delimiter
	if c.IncludeHeader {
		if e := w.Write(c.Columns); e != nil {
			return "", "", e
		}
	}
	for _, row := range rows {
		record := make([]string, 0, len(c.Columns))
		for _, column := range c.Columns {
			record = append(record, exportValue(column, row))
		}
		if e := w.Write(record); e != nil {
			return "", "", e
		}
	}
	w.Flush()
	if e := w.Error(); e != nil {
		return "", "", e
	}
	if len(rows) == 0 {
		return "", "", errors.New("payroll export has no rows")
	}
	return b.String(), prefix, nil
}

func PayrollFileName(prefix, reference string) string {
	component := fileComponent(strings.TrimSpace(reference))
	if component == "" {
		component = "payroll"
	}
	return prefix + "-" + component + ".csv"
}

func fileComponent(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "._-")
}

func exportValue(column string, r ExportRow) string {
	switch column {
	case "employee_number":
		return r.EmployeeNumber
	case "full_name":
		return r.FullName
	case "gross_minor":
		return strconv.FormatInt(r.GrossMinor, 10)
	case "other_deductions_minor":
		return strconv.FormatInt(r.OtherDeductionsMinor, 10)
	case "loan_deduction_minor":
		return strconv.FormatInt(r.LoanDeductionMinor, 10)
	case "total_deductions_minor":
		return strconv.FormatInt(r.OtherDeductionsMinor+r.LoanDeductionMinor, 10)
	case "net_minor":
		return strconv.FormatInt(r.NetMinor, 10)
	case "currency":
		return r.Currency
	case "payment_date":
		return r.PaymentDate
	case "payroll_reference":
		return r.PayrollReference
	}
	return ""
}
