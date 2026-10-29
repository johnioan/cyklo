package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// table is a simple grid for CSV/XLSX export. Cells are string or float64.
type table struct {
	sheet string
	rows  [][]any
}

func csvCell(v any, lang string) string {
	switch x := v.(type) {
	case float64:
		s := strconv.FormatFloat(x, 'f', 2, 64)
		if lang == "el" {
			s = strings.Replace(s, ".", ",", 1)
		}
		return s
	case string:
		return x
	}
	return ""
}

func writeCSV(w http.ResponseWriter, name, lang string, t table) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	cw := csv.NewWriter(&buf)
	cw.Comma = ';'
	for _, r := range t.rows {
		rec := make([]string, len(r))
		for i, c := range r {
			rec[i] = csvCell(c, lang)
		}
		_ = cw.Write(rec)
	}
	cw.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, name))
	_, _ = w.Write(buf.Bytes())
}

func xmlEsc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			if r >= 32 || r == '\n' || r == '\t' {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func colName(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+i%26)) + s
		i = i/26 - 1
	}
	return s
}

func buildXLSX(t table) []byte {
	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, r := range t.rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, ri+1)
		for ci, c := range r {
			ref := colName(ci) + strconv.Itoa(ri+1)
			switch x := c.(type) {
			case float64:
				fmt.Fprintf(&sheet, `<c r="%s" s="1"><v>%s</v></c>`, ref, strconv.FormatFloat(x, 'f', -1, 64))
			case string:
				style := ""
				if ri == 0 {
					style = ` s="2"`
				}
				fmt.Fprintf(&sheet, `<c r="%s" t="inlineStr"%s><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xmlEsc(x))
			}
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	files := []struct{ name, body string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + xmlEsc(t.sheet) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="1"><numFmt numFmtId="164" formatCode="#,##0.00"/></numFmts><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs></styleSheet>`},
		{"xl/worksheets/sheet1.xml", sheet.String()},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, _ := zw.Create(f.name)
		_, _ = w.Write([]byte(f.body))
	}
	_ = zw.Close()
	return buf.Bytes()
}

func writeTable(w http.ResponseWriter, r *http.Request, name, lang string, t table) {
	if r.URL.Query().Get("format") == "xlsx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
		_, _ = w.Write(buildXLSX(t))
		return
	}
	writeCSV(w, name, lang, t)
}

func (a *API) exportSubs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	cat, _ := strconv.Atoi(q.Get("cat"))
	text := strings.ToLower(strings.TrimSpace(q.Get("q")))
	t := today()
	var tb table
	var lang string
	a.st.View(func(d *Data) {
		lang = d.Settings.Lang
		tb.sheet = tr(lang, "sheet.subs")
		tb.rows = append(tb.rows, []any{tr(lang, "h.company"), tr(lang, "h.service"), tr(lang, "h.category"), tr(lang, "h.amountType"),
			tr(lang, "h.amount"), tr(lang, "h.frequency"), tr(lang, "h.start"), tr(lang, "h.next"), tr(lang, "h.method"), tr(lang, "h.card"),
			tr(lang, "h.status"), tr(lang, "h.remind"), tr(lang, "h.notes")})
		for i := range d.Subs {
			s := &d.Subs[i]
			if status != "" && status != "all" && d.subView(s, t).Status != status {
				continue
			}
			if cat != 0 && s.CategoryID != cat {
				continue
			}
			if text != "" && !strings.Contains(strings.ToLower(s.Company+" "+s.Service), text) {
				continue
			}
			info := nextInfo(s, t)
			var amount any = ""
			if s.Amount != nil {
				amount = *s.Amount
			}
			remind := tr(lang, "remind.0")
			if s.RemindDays == 1 {
				remind = tr(lang, "remind.1")
			} else if s.RemindDays > 1 {
				remind = trf(lang, "remind.n", s.RemindDays)
			}
			next := info.Date
			if s.Status != StatusActive {
				next = ""
			}
			tb.rows = append(tb.rows, []any{s.Company, s.Service, d.category(s.CategoryID), tr(lang, s.AmountType), amount,
				tr(lang, s.Frequency), s.StartDate, next, d.methodName(lang, s.Method), d.card(s.CardID), tr(lang, d.subView(s, t).Status), remind, s.Notes})
		}
	})
	writeTable(w, r, "cyklo-recurring-"+fmtDate(time.Now()), lang, tb)
}

func (a *API) exportReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var rep Report
	var tb table
	var lang string
	a.st.View(func(d *Data) {
		lang = d.Settings.Lang
		rep = d.buildReport(q.Get("view"), q.Get("period"), q.Get("date"), q.Get("status"))
		tb.sheet = tr(lang, "sheet.report")
		tb.rows = append(tb.rows, []any{tr(lang, "h.group"), tr(lang, "h.company"), tr(lang, "h.service"), tr(lang, "h.frequency"), tr(lang, "h.amount")})
		for _, row := range rep.Rows {
			label := row.Label
			if rep.View == "method" {
				label = d.methodName(lang, row.Method)
				if row.CardName != "" {
					label += " · " + row.CardName
				}
			}
			for _, it := range row.Items {
				var amount any = ""
				if it.Amount != nil {
					amount = *it.Amount
				}
				tb.rows = append(tb.rows, []any{label, it.Title, it.Service, tr(lang, it.Frequency), amount})
			}
		}
		tb.rows = append(tb.rows, []any{tr(lang, "h.total"), "", "", "", rep.Total})
	})
	writeTable(w, r, "cyklo-report-"+rep.View+"-"+rep.Date, lang, tb)
}
