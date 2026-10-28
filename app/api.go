package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type API struct {
	st *Store
	n  *Notifier
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func pathID(r *http.Request) int {
	id, _ := strconv.Atoi(r.PathValue("id"))
	return id
}

// ---------- views ----------

type SubView struct {
	Sub
	Title        string   `json:"title"`
	CategoryName string   `json:"categoryName"`
	CardName     string   `json:"cardName"`
	MethodName   string   `json:"methodName"`
	Next         NextInfo `json:"next"`
}

func (d *Data) subView(s *Sub, t time.Time) SubView {
	title := s.Company
	if s.Service != "" {
		title += " · " + s.Service
	}
	return SubView{Sub: *s, Title: title, CategoryName: d.category(s.CategoryID), CardName: d.card(s.CardID), MethodName: d.customMethod(s.Method), Next: nextInfo(s, t)}
}

type publicSettings struct {
	Settings
	PushoverToken string `json:"pushoverToken"`
	HasToken      bool   `json:"hasToken"`
}

func publicOf(s Settings) publicSettings {
	p := publicSettings{Settings: s, HasToken: s.PushoverToken != ""}
	p.PushoverToken = ""
	return p
}

// ---------- subscriptions ----------

func (a *API) listSubs(w http.ResponseWriter, r *http.Request) {
	t := today()
	var out []SubView
	a.st.View(func(d *Data) {
		out = make([]SubView, 0, len(d.Subs))
		for i := range d.Subs {
			out = append(out, d.subView(&d.Subs[i], t))
		}
	})
	sort.SliceStable(out, func(i, j int) bool {
		ni, nj := out[i].Next.Date, out[j].Next.Date
		if ni == "" {
			return false
		}
		if nj == "" {
			return true
		}
		return ni < nj
	})
	writeJSON(w, 200, out)
}

func (a *API) getSub(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	t := today()
	var v *SubView
	a.st.View(func(d *Data) {
		if s := d.sub(id); s != nil {
			sv := d.subView(s, t)
			v = &sv
		}
	})
	if v == nil {
		writeErr(w, 404, "not found")
		return
	}
	writeJSON(w, 200, v)
}

func validateSub(d *Data, s *Sub) error {
	s.Company = strings.TrimSpace(s.Company)
	s.Service = strings.TrimSpace(s.Service)
	s.Notes = strings.TrimSpace(s.Notes)
	if s.Company == "" {
		return errors.New("company")
	}
	if d.category(s.CategoryID) == "" {
		return errors.New("category")
	}
	if s.AmountType != TypeFixed && s.AmountType != TypeVariable {
		return errors.New("amountType")
	}
	if s.AmountType == TypeFixed {
		if s.Amount == nil || *s.Amount < 0 {
			return errors.New("amount")
		}
		v := round2(*s.Amount)
		s.Amount = &v
	} else {
		s.Amount = nil
	}
	if !validFreq(s.Frequency) {
		return errors.New("frequency")
	}
	if _, ok := parseDate(s.StartDate); !ok {
		return errors.New("startDate")
	}
	if s.ManualNext {
		if _, ok := parseDate(s.ManualDate); !ok {
			return errors.New("manualDate")
		}
	} else {
		s.ManualDate = ""
	}
	switch {
	case s.Method == MethodCard:
		if s.CardID != 0 && d.card(s.CardID) == "" {
			return errors.New("card")
		}
	case d.validMethod(s.Method):
		s.CardID = 0
	default:
		return errors.New("method")
	}
	switch s.RemindDays {
	case 0, 1, 3, 7:
	default:
		return errors.New("remindDays")
	}
	switch s.Status {
	case "":
		s.Status = StatusActive
	case StatusActive, StatusPaused, StatusCancelled:
	default:
		return errors.New("status")
	}
	return nil
}

func (a *API) createSub(w http.ResponseWriter, r *http.Request) {
	var in Sub
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	var created Sub
	err := a.st.Update(func(d *Data) error {
		in.ID = 0
		in.Status = StatusActive
		if err := validateSub(d, &in); err != nil {
			return err
		}
		in.ID = d.newID()
		if in.AmountType == TypeVariable {
			in.DueDate = initialDue(&in, today())
		}
		d.Subs = append(d.Subs, in)
		created = in
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, created)
}

func (a *API) updateSub(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in Sub
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	var out Sub
	err := a.st.Update(func(d *Data) error {
		cur := d.sub(id)
		if cur == nil {
			return errors.New("not found")
		}
		in.ID = id
		in.Status = cur.Status
		if err := validateSub(d, &in); err != nil {
			return err
		}
		if in.AmountType == TypeVariable {
			changed := cur.AmountType != TypeVariable || cur.StartDate != in.StartDate ||
				cur.Frequency != in.Frequency || in.ManualNext != cur.ManualNext || in.ManualDate != cur.ManualDate
			if changed || cur.DueDate == "" {
				in.DueDate = initialDue(&in, today())
			} else {
				in.DueDate = cur.DueDate
			}
		} else {
			in.DueDate = ""
		}
		*cur = in
		out = in
		return nil
	})
	if err != nil {
		if err.Error() == "not found" {
			writeErr(w, 404, err.Error())
		} else {
			writeErr(w, 400, err.Error())
		}
		return
	}
	writeJSON(w, 200, out)
}

func (a *API) setStatus(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	err := a.st.Update(func(d *Data) error {
		s := d.sub(id)
		if s == nil {
			return errors.New("not found")
		}
		switch in.Status {
		case StatusActive, StatusPaused, StatusCancelled:
			s.Status = in.Status
		default:
			return errors.New("status")
		}
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) deleteSub(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	err := a.st.Update(func(d *Data) error {
		idx := -1
		for i := range d.Subs {
			if d.Subs[i].ID == id {
				idx = i
			}
		}
		if idx < 0 {
			return errors.New("not found")
		}
		d.Subs = append(d.Subs[:idx], d.Subs[idx+1:]...)
		var keep []Payment
		for _, p := range d.Payments {
			if p.SubID != id {
				keep = append(keep, p)
			}
		}
		d.Payments = keep
		return nil
	})
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// recordBill stores the amount of the bill that is due and moves to the next one.
func (a *API) recordBill(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in struct {
		Amount float64 `json:"amount"`
	}
	if err := readJSON(r, &in); err != nil || in.Amount < 0 {
		writeErr(w, 400, "amount")
		return
	}
	err := a.st.Update(func(d *Data) error {
		s := d.sub(id)
		if s == nil {
			return errors.New("not found")
		}
		if s.AmountType != TypeVariable {
			return errors.New("not variable")
		}
		if _, ok := parseDate(s.DueDate); !ok {
			return errors.New("no due date")
		}
		v := round2(in.Amount)
		d.Payments = append(d.Payments, Payment{ID: d.newID(), SubID: id, Date: s.DueDate, Amount: &v})
		advanceDue(s)
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) skipBill(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	err := a.st.Update(func(d *Data) error {
		s := d.sub(id)
		if s == nil || s.AmountType != TypeVariable {
			return errors.New("not found")
		}
		advanceDue(s)
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- dashboard ----------

type CatShare struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Pct    int     `json:"pct"`
}

type MonthSum struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
}

func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	t := today()
	res := map[string]any{}
	a.st.View(func(d *Data) {
		cur := monthEnd(t.Year(), t.Month())
		monthTotal := sumEvents(d.eventsBetween(monthStart(t.Year(), t.Month()), cur, "active"))
		// next 12 months: only amounts that are known (fixed amounts)
		yearEnd := t.AddDate(1, 0, -1)
		yearTotal := sumEvents(d.eventsBetween(t, yearEnd, "active"))

		active := 0
		missing := 0
		type up struct {
			v SubView
		}
		var ups []SubView
		for i := range d.Subs {
			s := &d.Subs[i]
			if s.Status != StatusActive {
				continue
			}
			active++
			sv := d.subView(s, t)
			if s.AmountType == TypeVariable {
				if due, ok := parseDate(s.DueDate); ok && !due.After(cur) {
					missing++
				}
			}
			if sv.Next.Date != "" {
				ups = append(ups, sv)
			}
		}
		sort.SliceStable(ups, func(i, j int) bool { return ups[i].Next.Date < ups[j].Next.Date })
		if len(ups) > 6 {
			ups = ups[:6]
		}
		var next *SubView
		for i := range ups {
			if ups[i].Next.State != "missing" {
				next = &ups[i]
				break
			}
		}

		// last 12 months (including the current one)
		months := make([]MonthSum, 0, 12)
		catTotals := map[int]float64{}
		var catSum float64
		for k := 11; k >= 0; k-- {
			ms := time.Date(t.Year(), t.Month()-time.Month(k), 1, 0, 0, 0, 0, time.UTC)
			me := monthEnd(ms.Year(), ms.Month())
			ev := d.eventsBetween(ms, me, "active")
			months = append(months, MonthSum{Month: ms.Format("2006-01"), Amount: sumEvents(ev)})
			for _, e := range ev {
				if e.Amount == nil {
					continue
				}
				if s := d.sub(e.SubID); s != nil {
					catTotals[s.CategoryID] += *e.Amount
					catSum += *e.Amount
				}
			}
		}
		var cats []CatShare
		for _, c := range d.Categories {
			if v := catTotals[c.ID]; v > 0 {
				cats = append(cats, CatShare{Name: c.Name, Amount: round2(v), Pct: int(v/catSum*100 + 0.5)})
			}
		}
		sort.SliceStable(cats, func(i, j int) bool { return cats[i].Amount > cats[j].Amount })

		res["monthTotal"] = monthTotal
		res["yearTotal"] = yearTotal
		res["activeCount"] = active
		res["missingCount"] = missing
		res["upcoming"] = ups
		res["next"] = next
		res["categories"] = cats
		res["months"] = months
		res["subCount"] = len(d.Subs)
		res["settings"] = publicOf(d.Settings)
	})
	writeJSON(w, 200, res)
}

// ---------- reports ----------

type ReportItem struct {
	SubID     int      `json:"subId"`
	Title     string   `json:"title"`
	Service   string   `json:"service"`
	Frequency string   `json:"frequency"`
	Amount    *float64 `json:"amount"`
}

type ReportRow struct {
	Key      string       `json:"key"`
	Label    string       `json:"label"`
	Method   string       `json:"method,omitempty"`
	CardName string       `json:"cardName,omitempty"`
	Amount   float64      `json:"amount"`
	Pct      int          `json:"pct"`
	SubCount int          `json:"subCount"`
	Items    []ReportItem `json:"items"`
}

type Report struct {
	View    string      `json:"view"`
	Period  string      `json:"period"`
	Date    string      `json:"date"`
	Status  string      `json:"status"`
	Total   float64     `json:"total"`
	Pending int         `json:"pending"`
	Rows    []ReportRow `json:"rows"`
}

func periodRange(period, date string, t time.Time) (time.Time, time.Time, string) {
	if period == "year" {
		y := t.Year()
		if v, err := strconv.Atoi(date); err == nil && v > 1990 && v < 2200 {
			y = v
		}
		return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC), strconv.Itoa(y)
	}
	y, m := t.Year(), t.Month()
	if tm, err := time.Parse("2006-01", date); err == nil {
		y, m = tm.Year(), tm.Month()
	}
	return monthStart(y, m), monthEnd(y, m), time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
}

func (d *Data) buildReport(view, period, date, status string) Report {
	t := today()
	from, to, norm := periodRange(period, date, t)
	if status != "all" && status != "activepaused" {
		status = "active"
	}
	if view != "company" && view != "category" {
		view = "method"
	}
	if period != "year" {
		period = "month"
	}
	rep := Report{View: view, Period: period, Date: norm, Status: status, Rows: []ReportRow{}}
	ev := d.eventsBetween(from, to, status)
	rows := map[string]*ReportRow{}
	order := []string{}
	itemIdx := map[string]int{} // rowKey|subID -> index
	for _, e := range ev {
		s := d.sub(e.SubID)
		if s == nil {
			continue
		}
		if e.Amount == nil {
			rep.Pending++
		}
		var key, label, method, card string
		switch view {
		case "company":
			key, label = "co:"+strings.ToLower(s.Company), s.Company
		case "category":
			key, label = "cat:"+strconv.Itoa(s.CategoryID), d.category(s.CategoryID)
		default:
			method = s.Method
			if s.Method == MethodCard {
				card = d.card(s.CardID)
			}
			key = "m:" + s.Method + ":" + strconv.Itoa(s.CardID)
			label = d.customMethod(s.Method)
			if label == "" {
				label = s.Method
			}
		}
		row, ok := rows[key]
		if !ok {
			row = &ReportRow{Key: key, Label: label, Method: method, CardName: card, Items: []ReportItem{}}
			rows[key] = row
			order = append(order, key)
		}
		ik := key + "|" + strconv.Itoa(s.ID)
		idx, ok := itemIdx[ik]
		if !ok {
			title := s.Company
			row.Items = append(row.Items, ReportItem{SubID: s.ID, Title: title, Service: s.Service, Frequency: s.Frequency})
			idx = len(row.Items) - 1
			itemIdx[ik] = idx
			row.SubCount++
		}
		if e.Amount != nil {
			cur := 0.0
			if row.Items[idx].Amount != nil {
				cur = *row.Items[idx].Amount
			}
			v := round2(cur + *e.Amount)
			row.Items[idx].Amount = &v
			row.Amount = round2(row.Amount + *e.Amount)
			rep.Total = round2(rep.Total + *e.Amount)
		}
	}
	for _, k := range order {
		rep.Rows = append(rep.Rows, *rows[k])
	}
	sort.SliceStable(rep.Rows, func(i, j int) bool { return rep.Rows[i].Amount > rep.Rows[j].Amount })
	for i := range rep.Rows {
		if rep.Total > 0 {
			rep.Rows[i].Pct = int(rep.Rows[i].Amount/rep.Total*100 + 0.5)
		}
	}
	return rep
}

func (a *API) report(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var rep Report
	a.st.View(func(d *Data) {
		rep = d.buildReport(q.Get("view"), q.Get("period"), q.Get("date"), q.Get("status"))
	})
	writeJSON(w, 200, rep)
}

// ---------- calendar ----------

type CalEvent struct {
	SubID    int      `json:"subId"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	Amount   *float64 `json:"amount"`
	State    string   `json:"state"`
	Variable bool     `json:"variable"`
}

func (a *API) calendar(w http.ResponseWriter, r *http.Request) {
	t := today()
	y, m := t.Year(), t.Month()
	if tm, err := time.Parse("2006-01", r.URL.Query().Get("month")); err == nil {
		y, m = tm.Year(), tm.Month()
	}
	from, to := monthStart(y, m), monthEnd(y, m)
	out := map[string]any{}
	a.st.View(func(d *Data) {
		ev := d.eventsBetween(from, to, "active")
		list := make([]CalEvent, 0, len(ev))
		pending := 0
		for _, e := range ev {
			s := d.sub(e.SubID)
			if s == nil {
				continue
			}
			sv := d.subView(s, t)
			if e.Amount == nil {
				pending++
			}
			list = append(list, CalEvent{SubID: s.ID, Title: sv.Title, Date: e.Date, Amount: e.Amount, State: e.State, Variable: s.AmountType == TypeVariable})
		}
		out["month"] = from.Format("2006-01")
		out["events"] = list
		out["total"] = sumEvents(ev)
		out["pending"] = pending
		out["today"] = fmtDate(t)
	})
	writeJSON(w, 200, out)
}

// ---------- categories and cards ----------

func looksLikeCardNumber(s string) bool {
	n := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			n++
		}
	}
	return n >= 8
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 60 {
		s = string([]rune(s)[:60])
	}
	return s
}

func (a *API) listLists(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	a.st.View(func(d *Data) {
		catUse := map[int]int{}
		cardUse := map[int]int{}
		for _, s := range d.Subs {
			catUse[s.CategoryID]++
			if s.CardID != 0 {
				cardUse[s.CardID]++
			}
		}
		type item struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		methodUse := map[string]int{}
		for _, s := range d.Subs {
			methodUse[s.Method]++
		}
		var cats, cards, methods []item
		for _, c := range d.Categories {
			cats = append(cats, item{c.ID, c.Name, catUse[c.ID]})
		}
		for _, c := range d.Cards {
			cards = append(cards, item{c.ID, c.Name, cardUse[c.ID]})
		}
		for _, c := range d.Methods {
			methods = append(methods, item{c.ID, c.Name, methodUse["c"+strconv.Itoa(c.ID)]})
		}
		if methods == nil {
			methods = []item{}
		}
		if cats == nil {
			cats = []item{}
		}
		if cards == nil {
			cards = []item{}
		}
		out["categories"] = cats
		out["cards"] = cards
		out["methods"] = methods
		out["settings"] = publicOf(d.Settings)
	})
	writeJSON(w, 200, out)
}

func (a *API) addCategory(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name string }
	_ = readJSON(r, &in)
	name := cleanName(in.Name)
	err := a.st.Update(func(d *Data) error {
		if name == "" {
			return errors.New("name")
		}
		for _, c := range d.Categories {
			if strings.EqualFold(c.Name, name) {
				return errors.New("exists")
			}
		}
		d.Categories = append(d.Categories, Category{ID: d.newID(), Name: name})
		d.Settings.DefaultCats = false
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var id int
	a.st.View(func(d *Data) { id = d.Categories[len(d.Categories)-1].ID })
	writeJSON(w, 201, map[string]any{"ok": true, "id": id})
}

func (a *API) renameCategory(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in struct{ Name string }
	_ = readJSON(r, &in)
	name := cleanName(in.Name)
	err := a.st.Update(func(d *Data) error {
		if name == "" {
			return errors.New("name")
		}
		for i := range d.Categories {
			if d.Categories[i].ID != id && strings.EqualFold(d.Categories[i].Name, name) {
				return errors.New("exists")
			}
		}
		for i := range d.Categories {
			if d.Categories[i].ID == id {
				d.Categories[i].Name = name
				d.Settings.DefaultCats = false
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	moveTo, _ := strconv.Atoi(r.URL.Query().Get("moveTo"))
	err := a.st.Update(func(d *Data) error {
		if len(d.Categories) <= 1 {
			return errors.New("last")
		}
		used := false
		for _, s := range d.Subs {
			if s.CategoryID == id {
				used = true
			}
		}
		if used {
			if moveTo == id || d.category(moveTo) == "" {
				return errors.New("moveTo")
			}
			for i := range d.Subs {
				if d.Subs[i].CategoryID == id {
					d.Subs[i].CategoryID = moveTo
				}
			}
		}
		idx := -1
		for i := range d.Categories {
			if d.Categories[i].ID == id {
				idx = i
			}
		}
		if idx < 0 {
			return errors.New("not found")
		}
		d.Categories = append(d.Categories[:idx], d.Categories[idx+1:]...)
		d.Settings.DefaultCats = false
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) addCard(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name string }
	_ = readJSON(r, &in)
	name := cleanName(in.Name)
	err := a.st.Update(func(d *Data) error {
		if name == "" {
			return errors.New("name")
		}
		if looksLikeCardNumber(name) {
			return errors.New("cardnumber")
		}
		for _, c := range d.Cards {
			if strings.EqualFold(c.Name, name) {
				return errors.New("exists")
			}
		}
		d.Cards = append(d.Cards, Card{ID: d.newID(), Name: name})
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

func (a *API) renameCard(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in struct{ Name string }
	_ = readJSON(r, &in)
	name := cleanName(in.Name)
	err := a.st.Update(func(d *Data) error {
		if name == "" {
			return errors.New("name")
		}
		if looksLikeCardNumber(name) {
			return errors.New("cardnumber")
		}
		for i := range d.Cards {
			if d.Cards[i].ID != id && strings.EqualFold(d.Cards[i].Name, name) {
				return errors.New("exists")
			}
		}
		for i := range d.Cards {
			if d.Cards[i].ID == id {
				d.Cards[i].Name = name
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) deleteCard(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	err := a.st.Update(func(d *Data) error {
		idx := -1
		for i := range d.Cards {
			if d.Cards[i].ID == id {
				idx = i
			}
		}
		if idx < 0 {
			return errors.New("not found")
		}
		d.Cards = append(d.Cards[:idx], d.Cards[idx+1:]...)
		for i := range d.Subs {
			if d.Subs[i].CardID == id {
				d.Subs[i].CardID = 0
			}
		}
		return nil
	})
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- settings ----------

func (a *API) getSettings(w http.ResponseWriter, r *http.Request) {
	var p publicSettings
	a.st.View(func(d *Data) { p = publicOf(d.Settings) })
	writeJSON(w, 200, p)
}

func (a *API) putSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Settings
		ClearToken bool `json:"clearToken"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	err := a.st.Update(func(d *Data) error {
		s := &d.Settings
		if in.Lang != "el" && in.Lang != "en" {
			return errors.New("lang")
		}
		switch in.Currency {
		case "EUR", "USD", "GBP", "CHF":
		default:
			return errors.New("currency")
		}
		switch in.RemindDays {
		case 1, 3, 7:
		default:
			return errors.New("remindDays")
		}
		if in.SendHour < 0 || in.SendHour > 23 {
			return errors.New("sendHour")
		}
		if in.MissingRepeatDays == 0 {
			in.MissingRepeatDays = s.RepeatDays()
		}
		if in.MissingRepeatMax == 0 {
			in.MissingRepeatMax = s.RepeatMax()
		}
		switch in.MissingRepeatDays {
		case 1, 2, 3, 5, 7:
		default:
			return errors.New("missingRepeatDays")
		}
		switch in.MissingRepeatMax {
		case 1, 2, 3, 5:
		default:
			return errors.New("missingRepeatMax")
		}
		if in.AuthMode != "off" && in.AuthMode != "dsm" {
			return errors.New("authMode")
		}
		if in.Lang != s.Lang && s.DefaultCats {
			from, to := greekCats, englishCats
			if s.Lang == "en" {
				from, to = englishCats, greekCats
			}
			for i := range d.Categories {
				for k, n := range from {
					if d.Categories[i].Name == n {
						d.Categories[i].Name = to[k]
					}
				}
			}
		}
		s.Lang, s.Currency, s.RemindDays, s.SendHour = in.Lang, in.Currency, in.RemindDays, in.SendHour
		s.MissingBillReminder = in.MissingBillReminder
		s.MissingRepeatDays, s.MissingRepeatMax = in.MissingRepeatDays, in.MissingRepeatMax
		s.EmailEnabled, s.PushEnabled = in.EmailEnabled, in.PushEnabled
		s.PushoverUser = strings.TrimSpace(in.PushoverUser)
		if tok := strings.TrimSpace(in.PushoverToken); tok != "" {
			s.PushoverToken = tok
		}
		if in.ClearToken {
			s.PushoverToken = ""
		}
		s.OnboardingDone = in.OnboardingDone
		s.AuthMode = in.AuthMode
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	a.getSettings(w, r)
}

func (a *API) testNotify(w http.ResponseWriter, r *http.Request) {
	res := a.n.SendTest()
	writeJSON(w, 200, res)
}

// ---------- backup ----------

func (a *API) backup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="cyklo-backup-`+fmtDate(today())+`.json"`)
	_, _ = w.Write(a.st.Snapshot())
}

func (a *API) restore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	d := &Data{}
	if err := json.NewDecoder(r.Body).Decode(d); err != nil || d.NextID == 0 || len(d.Categories) == 0 {
		writeErr(w, 400, "invalid backup")
		return
	}
	if err := a.st.Replace(d); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard", a.dashboard)
	mux.HandleFunc("GET /api/subs", a.listSubs)
	mux.HandleFunc("POST /api/subs", a.createSub)
	mux.HandleFunc("GET /api/subs/{id}", a.getSub)
	mux.HandleFunc("PUT /api/subs/{id}", a.updateSub)
	mux.HandleFunc("DELETE /api/subs/{id}", a.deleteSub)
	mux.HandleFunc("POST /api/subs/{id}/status", a.setStatus)
	mux.HandleFunc("POST /api/subs/{id}/record", a.recordBill)
	mux.HandleFunc("POST /api/subs/{id}/skip", a.skipBill)
	mux.HandleFunc("GET /api/report", a.report)
	mux.HandleFunc("GET /api/calendar", a.calendar)
	mux.HandleFunc("GET /api/lists", a.listLists)
	mux.HandleFunc("POST /api/categories", a.addCategory)
	mux.HandleFunc("PUT /api/categories/{id}", a.renameCategory)
	mux.HandleFunc("DELETE /api/categories/{id}", a.deleteCategory)
	mux.HandleFunc("POST /api/methods", a.addMethod)
	mux.HandleFunc("PUT /api/methods/{id}", a.renameMethod)
	mux.HandleFunc("DELETE /api/methods/{id}", a.deleteMethod)
	mux.HandleFunc("POST /api/cards", a.addCard)
	mux.HandleFunc("PUT /api/cards/{id}", a.renameCard)
	mux.HandleFunc("DELETE /api/cards/{id}", a.deleteCard)
	mux.HandleFunc("GET /api/settings", a.getSettings)
	mux.HandleFunc("PUT /api/settings", a.putSettings)
	mux.HandleFunc("POST /api/notify/test", a.testNotify)
	mux.HandleFunc("GET /api/backup", a.backup)
	mux.HandleFunc("POST /api/restore", a.restore)
	mux.HandleFunc("GET /api/export/subs", a.exportSubs)
	mux.HandleFunc("GET /api/export/report", a.exportReport)
}

// ---------- user defined payment methods ----------

func (a *API) addMethod(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name string }
	_ = readJSON(r, &in)
	name := cleanName(in.Name)
	err := a.st.Update(func(d *Data) error {
		if name == "" {
			return errors.New("name")
		}
		if looksLikeCardNumber(name) {
			return errors.New("cardnumber")
		}
		for _, m := range d.Methods {
			if strings.EqualFold(m.Name, name) {
				return errors.New("exists")
			}
		}
		for _, l := range []string{"el", "en"} {
			for _, b := range []string{MethodCard, MethodPayPal, MethodApplePay} {
				if strings.EqualFold(tr(l, b), name) {
					return errors.New("exists")
				}
			}
		}
		d.Methods = append(d.Methods, Category{ID: d.newID(), Name: name})
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var id int
	a.st.View(func(d *Data) { id = d.Methods[len(d.Methods)-1].ID })
	writeJSON(w, 201, map[string]any{"ok": true, "id": id, "key": "c" + strconv.Itoa(id)})
}

func (a *API) renameMethod(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in struct{ Name string }
	_ = readJSON(r, &in)
	name := cleanName(in.Name)
	err := a.st.Update(func(d *Data) error {
		if name == "" {
			return errors.New("name")
		}
		if looksLikeCardNumber(name) {
			return errors.New("cardnumber")
		}
		for i := range d.Methods {
			if d.Methods[i].ID != id && strings.EqualFold(d.Methods[i].Name, name) {
				return errors.New("exists")
			}
		}
		for i := range d.Methods {
			if d.Methods[i].ID == id {
				d.Methods[i].Name = name
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) deleteMethod(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	key := "c" + strconv.Itoa(id)
	moveTo := r.URL.Query().Get("moveTo")
	err := a.st.Update(func(d *Data) error {
		idx := -1
		for i := range d.Methods {
			if d.Methods[i].ID == id {
				idx = i
			}
		}
		if idx < 0 {
			return errors.New("not found")
		}
		used := false
		for _, s := range d.Subs {
			if s.Method == key {
				used = true
			}
		}
		if used {
			if moveTo == key || !d.validMethod(moveTo) {
				return errors.New("moveTo")
			}
			for i := range d.Subs {
				if d.Subs[i].Method == key {
					d.Subs[i].Method = moveTo
					d.Subs[i].CardID = 0
				}
			}
		}
		d.Methods = append(d.Methods[:idx], d.Methods[idx+1:]...)
		return nil
	})
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
