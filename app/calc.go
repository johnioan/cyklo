package main

import (
	"math"
	"sort"
	"strconv"
	"time"
)

const dateLayout = "2006-01-02"

func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse(dateLayout, s)
	return t, err == nil
}

func fmtDate(t time.Time) string { return t.Format(dateLayout) }

// today returns the current local date as a UTC-midnight time (pure date).
func today() time.Time {
	n := nowFunc()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func monthsPerStep(freq string) int {
	switch freq {
	case FreqMonthly:
		return 1
	case FreqQuarterly:
		return 3
	case FreqSemiannual:
		return 6
	case FreqAnnual:
		return 12
	}
	return 0 // weekly
}

func validFreq(f string) bool {
	switch f {
	case FreqWeekly, FreqMonthly, FreqQuarterly, FreqSemiannual, FreqAnnual:
		return true
	}
	return false
}

func daysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// occAt returns the k-th occurrence (k >= 0) of a schedule that starts at anchor.
// For monthly based frequencies the day of the anchor is kept and, in shorter
// months, the last day of the month is used (rule: 29, 30, 31 -> last day).
func occAt(anchor time.Time, freq string, k int) time.Time {
	step := monthsPerStep(freq)
	if step == 0 {
		return anchor.AddDate(0, 0, 7*k)
	}
	total := step * k
	first := time.Date(anchor.Year(), anchor.Month()+time.Month(total), 1, 0, 0, 0, 0, time.UTC)
	day := anchor.Day()
	if last := daysInMonth(first.Year(), first.Month()); day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
}

// nextOnOrAfter returns the first occurrence of the schedule that is >= d.
func nextOnOrAfter(anchor time.Time, freq string, d time.Time) time.Time {
	if !d.After(anchor) {
		return anchor
	}
	step := monthsPerStep(freq)
	k := 0
	if step == 0 {
		diff := int(d.Sub(anchor).Hours() / 24)
		k = diff / 7
	} else {
		md := (d.Year()-anchor.Year())*12 + int(d.Month()) - int(anchor.Month())
		k = md/step - 1
		if k < 0 {
			k = 0
		}
	}
	for i := 0; i < 5000; i++ {
		o := occAt(anchor, freq, k)
		if !o.Before(d) {
			return o
		}
		k++
	}
	return occAt(anchor, freq, k)
}

// occurrencesBetween lists occurrences in [from, to] (inclusive) of a schedule.
func occurrencesBetween(anchor time.Time, freq string, from, to time.Time) []time.Time {
	var out []time.Time
	if to.Before(anchor) {
		return out
	}
	o := nextOnOrAfter(anchor, freq, from)
	k := 0
	// find k of o by walking from a close estimate
	step := monthsPerStep(freq)
	if step == 0 {
		k = int(o.Sub(anchor).Hours()/24) / 7
	} else {
		md := (o.Year()-anchor.Year())*12 + int(o.Month()) - int(anchor.Month())
		k = md / step
	}
	for i := 0; i < 5000; i++ {
		o = occAt(anchor, freq, k)
		if o.After(to) {
			break
		}
		if !o.Before(from) && !o.Before(anchor) {
			out = append(out, o)
		}
		k++
	}
	return out
}

// anchorFor returns the date from which the "next payment" is calculated.
func anchorFor(s *Sub) (time.Time, bool) {
	if s.ManualNext {
		if t, ok := parseDate(s.ManualDate); ok {
			return t, true
		}
	}
	return parseDate(s.StartDate)
}

// NextInfo describes the next payment of a subscription.
type NextInfo struct {
	Date  string `json:"date"`  // empty when not known
	State string `json:"state"` // "upcoming", "awaiting" (variable, not yet due), "missing" (variable, overdue)
	Days  int    `json:"days"`  // days from today (negative when overdue)
}

func nextInfo(s *Sub, t time.Time) NextInfo {
	if s.AmountType == TypeVariable {
		due, ok := parseDate(s.DueDate)
		if !ok {
			return NextInfo{}
		}
		days := int(due.Sub(t).Hours() / 24)
		st := "awaiting"
		if due.Before(t) {
			st = "missing"
		}
		return NextInfo{Date: fmtDate(due), State: st, Days: days}
	}
	anchor, ok := anchorFor(s)
	if !ok || !validFreq(s.Frequency) {
		return NextInfo{}
	}
	n := nextOnOrAfter(anchor, s.Frequency, t)
	return NextInfo{Date: fmtDate(n), State: "upcoming", Days: int(n.Sub(t).Hours() / 24)}
}

// initialDue returns the due date for a new or changed variable-amount obligation.
func initialDue(s *Sub, t time.Time) string {
	anchor, ok := anchorFor(s)
	if !ok || !validFreq(s.Frequency) {
		return ""
	}
	if s.ManualNext {
		return fmtDate(anchor)
	}
	return fmtDate(nextOnOrAfter(anchor, s.Frequency, t))
}

// advanceDue moves the due date of a variable-amount obligation to the next occurrence.
func advanceDue(s *Sub) {
	cur, ok := parseDate(s.DueDate)
	if !ok {
		return
	}
	start, ok := parseDate(s.StartDate)
	if !ok || !validFreq(s.Frequency) {
		return
	}
	anchor := start
	if s.ManualNext {
		if m, ok := parseDate(s.ManualDate); ok {
			anchor = m
		}
	}
	s.DueDate = fmtDate(nextOnOrAfter(anchor, s.Frequency, cur.AddDate(0, 0, 1)))
}

// Event is one charge (past, expected or missing) of a subscription.
type Event struct {
	SubID  int      `json:"subId"`
	Date   string   `json:"date"`
	Amount *float64 `json:"amount"`
	State  string   `json:"state"` // "charged", "upcoming", "paid", "awaiting", "missing"
}

func statusAllowed(filter string, status string) bool {
	switch filter {
	case "all":
		return true
	case "activepaused":
		return status == StatusActive || status == StatusPaused
	default:
		return status == StatusActive
	}
}

// eventsBetween lists all charges between from and to (inclusive).
func (d *Data) eventsBetween(from, to time.Time, statusFilter string) []Event {
	t := today()
	var out []Event
	for i := range d.Subs {
		s := &d.Subs[i]
		if !statusAllowed(statusFilter, s.Status) {
			continue
		}
		if s.AmountType == TypeFixed {
			start, ok := parseDate(s.StartDate)
			if !ok || !validFreq(s.Frequency) {
				continue
			}
			for _, o := range occurrencesBetween(start, s.Frequency, from, to) {
				st := "upcoming"
				if o.Before(t) {
					st = "charged"
				}
				out = append(out, Event{SubID: s.ID, Date: fmtDate(o), Amount: s.Amount, State: st})
			}
			continue
		}
		for _, p := range d.Payments {
			if p.SubID != s.ID {
				continue
			}
			pd, ok := parseDate(p.Date)
			if ok && !pd.Before(from) && !pd.After(to) {
				out = append(out, Event{SubID: s.ID, Date: p.Date, Amount: p.Amount, State: "paid"})
			}
		}
		if due, ok := parseDate(s.DueDate); ok && !due.Before(from) && !due.After(to) {
			st := "awaiting"
			if due.Before(t) {
				st = "missing"
			}
			out = append(out, Event{SubID: s.ID, Date: s.DueDate, State: st})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func sumEvents(ev []Event) float64 {
	var s float64
	for _, e := range ev {
		if e.Amount != nil {
			s += *e.Amount
		}
	}
	return round2(s)
}

func monthStart(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func monthEnd(y int, m time.Month) time.Time {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
}

func (d *Data) sub(id int) *Sub {
	for i := range d.Subs {
		if d.Subs[i].ID == id {
			return &d.Subs[i]
		}
	}
	return nil
}

func (d *Data) category(id int) string {
	for _, c := range d.Categories {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

func (d *Data) card(id int) string {
	for _, c := range d.Cards {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// customMethodID returns the id of a user defined payment method ("c12" -> 12).
func customMethodID(m string) (int, bool) {
	if len(m) < 2 || m[0] != 'c' {
		return 0, false
	}
	id, err := strconv.Atoi(m[1:])
	return id, err == nil && id > 0
}

func (d *Data) customMethod(m string) string {
	id, ok := customMethodID(m)
	if !ok {
		return ""
	}
	for _, c := range d.Methods {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

func (d *Data) validMethod(m string) bool {
	switch m {
	case MethodCard, MethodPayPal, MethodApplePay:
		return true
	}
	return d.customMethod(m) != ""
}

// methodName is the display name of a payment method in the given language.
func (d *Data) methodName(lang, m string) string {
	if n := d.customMethod(m); n != "" {
		return n
	}
	return tr(lang, m)
}
