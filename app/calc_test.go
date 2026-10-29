package main

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, ok := parseDate(s)
	if !ok {
		panic(s)
	}
	return t
}

func TestOccAtClampsToMonthEnd(t *testing.T) {
	a := d("2026-01-31")
	cases := map[int]string{0: "2026-01-31", 1: "2026-02-28", 2: "2026-03-31", 3: "2026-04-30", 12: "2027-01-31"}
	for k, want := range cases {
		if got := fmtDate(occAt(a, FreqMonthly, k)); got != want {
			t.Errorf("k=%d got %s want %s", k, got, want)
		}
	}
	// leap year
	if got := fmtDate(occAt(d("2027-12-31"), FreqMonthly, 2)); got != "2028-02-29" {
		t.Errorf("leap got %s", got)
	}
	// annual on 29 Feb
	if got := fmtDate(occAt(d("2028-02-29"), FreqAnnual, 1)); got != "2029-02-28" {
		t.Errorf("annual got %s", got)
	}
	if got := fmtDate(occAt(d("2028-02-29"), FreqAnnual, 4)); got != "2032-02-29" {
		t.Errorf("annual leap got %s", got)
	}
}

func TestNextOnOrAfter(t *testing.T) {
	cases := []struct{ anchor, freq, from, want string }{
		{"2026-01-31", FreqMonthly, "2026-02-01", "2026-02-28"},
		{"2026-01-31", FreqMonthly, "2026-02-28", "2026-02-28"},
		{"2026-01-31", FreqMonthly, "2026-03-01", "2026-03-31"},
		{"2026-10-26", FreqMonthly, "2026-10-23", "2026-10-26"},
		{"2026-10-26", FreqMonthly, "2026-10-27", "2026-11-26"},
		{"2025-03-15", FreqQuarterly, "2026-10-23", "2026-12-15"},
		{"2025-03-15", FreqSemiannual, "2026-10-23", "2027-03-15"},
		{"2020-11-30", FreqAnnual, "2026-10-23", "2026-11-30"},
		{"2026-10-01", FreqWeekly, "2026-10-23", "2026-10-29"},
		{"2026-10-01", FreqWeekly, "2026-10-22", "2026-10-22"},
		{"2027-01-10", FreqMonthly, "2026-10-23", "2027-01-10"}, // start in the future
	}
	for _, c := range cases {
		got := fmtDate(nextOnOrAfter(d(c.anchor), c.freq, d(c.from)))
		if got != c.want {
			t.Errorf("%v: got %s", c, got)
		}
	}
}

func TestOccurrencesBetween(t *testing.T) {
	got := occurrencesBetween(d("2026-01-31"), FreqMonthly, d("2026-02-01"), d("2026-04-30"))
	want := []string{"2026-02-28", "2026-03-31", "2026-04-30"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if fmtDate(got[i]) != want[i] {
			t.Errorf("got %v", got)
		}
	}
	// nothing before the start date
	if n := len(occurrencesBetween(d("2026-06-10"), FreqMonthly, d("2026-01-01"), d("2026-05-31"))); n != 0 {
		t.Errorf("expected none, got %d", n)
	}
	w := occurrencesBetween(d("2026-10-01"), FreqWeekly, d("2026-10-01"), d("2026-10-31"))
	if len(w) != 5 {
		t.Errorf("weekly got %d", len(w))
	}
}

func fixedNow(s string) func() {
	old := nowFunc
	tt := d(s).Add(10 * time.Hour)
	nowFunc = func() time.Time { return time.Date(tt.Year(), tt.Month(), tt.Day(), 10, 0, 0, 0, time.Local) }
	return func() { nowFunc = old }
}

func TestVariableDueFlow(t *testing.T) {
	defer fixedNow("2026-10-23")()
	f := 12.5
	s := &Sub{ID: 1, AmountType: TypeVariable, Frequency: FreqMonthly, StartDate: "2026-08-02", Status: StatusActive}
	s.DueDate = initialDue(s, today())
	if s.DueDate != "2026-11-02" {
		t.Fatalf("due %s", s.DueDate)
	}
	info := nextInfo(s, today())
	if info.State != "awaiting" || info.Days != 10 {
		t.Errorf("info %+v", info)
	}
	// overdue
	s.DueDate = "2026-10-02"
	if nextInfo(s, today()).State != "missing" {
		t.Errorf("expected missing")
	}
	advanceDue(s)
	if s.DueDate != "2026-11-02" {
		t.Errorf("advance got %s", s.DueDate)
	}
	_ = f
}

func TestEventsAndTotals(t *testing.T) {
	defer fixedNow("2026-10-23")()
	a := 10.0
	b := 100.0
	dd := &Data{
		Subs: []Sub{
			{ID: 1, Company: "A", AmountType: TypeFixed, Amount: &a, Frequency: FreqMonthly, StartDate: "2026-01-15", Status: StatusActive},
			{ID: 2, Company: "B", AmountType: TypeFixed, Amount: &b, Frequency: FreqAnnual, StartDate: "2025-11-30", Status: StatusActive},
			{ID: 3, Company: "Power", AmountType: TypeVariable, Frequency: FreqMonthly, StartDate: "2026-01-02", DueDate: "2026-11-02", Status: StatusActive},
			{ID: 4, Company: "Paused", AmountType: TypeFixed, Amount: &a, Frequency: FreqMonthly, StartDate: "2026-01-15", Status: StatusPaused},
		},
	}
	v := 55.55
	dd.Payments = []Payment{{ID: 9, SubID: 3, Date: "2026-10-02", Amount: &v}}
	ev := dd.eventsBetween(d("2026-10-01"), d("2026-10-31"), "active")
	if got := sumEvents(ev); got != 65.55 {
		t.Errorf("october total %v (%v)", got, ev)
	}
	ev = dd.eventsBetween(d("2026-11-01"), d("2026-11-30"), "active")
	// A (10) + B annual on 30 Nov (100); Power is awaiting without amount
	if got := sumEvents(ev); got != 110 {
		t.Errorf("november total %v", got)
	}
	pending := 0
	for _, e := range ev {
		if e.Amount == nil {
			pending++
		}
	}
	if pending != 1 {
		t.Errorf("pending %d", pending)
	}
	ev = dd.eventsBetween(d("2026-10-01"), d("2026-10-31"), "activepaused")
	if got := sumEvents(ev); got != 75.55 {
		t.Errorf("with paused %v", got)
	}
	rep := dd.buildReport("company", "month", "2026-10", "active")
	if rep.Total != 65.55 || len(rep.Rows) != 2 {
		t.Errorf("report %+v", rep)
	}
}

func instSub(start string, n int) *Sub {
	a := 95.0
	return &Sub{ID: 1, Company: "Public", AmountType: TypeInstallments, Amount: &a, Frequency: FreqMonthly, StartDate: start, Installments: n, Status: StatusActive}
}

func TestInstallmentProgress(t *testing.T) {
	s := instSub("2026-01-15", 12)
	cases := []struct {
		now       string
		wantDate  string
		wantIdx   int
		wantPaid  int
		wantState string
		wantDays  int
	}{
		{"2026-01-10", "2026-01-15", 1, 0, "upcoming", 5},
		{"2026-01-15", "2026-01-15", 1, 0, "upcoming", 0},
		{"2026-01-16", "2026-02-15", 2, 1, "upcoming", 30},
		{"2026-10-28", "2026-11-15", 11, 10, "upcoming", 18},
		{"2026-12-15", "2026-12-15", 12, 11, "upcoming", 0},
		{"2026-12-16", "", 0, 12, "completed", 0},
		{"2030-01-01", "", 0, 12, "completed", 0},
	}
	for _, c := range cases {
		ni := nextInfo(s, d(c.now))
		if ni.Date != c.wantDate || ni.Index != c.wantIdx || ni.Paid != c.wantPaid || ni.State != c.wantState || ni.Days != c.wantDays {
			t.Errorf("%s: got %+v", c.now, ni)
		}
	}
}

func TestInstallmentEventsAndCompletion(t *testing.T) {
	defer fixedNow("2026-10-28")()
	data := defaultData()
	data.Subs = []Sub{*instSub("2026-01-31", 3)} // 31 Jan, 28 Feb, 31 Mar
	ev := data.eventsBetween(d("2026-01-01"), d("2026-12-31"), "active")
	if len(ev) != 3 || ev[0].Date != "2026-01-31" || ev[1].Date != "2026-02-28" || ev[2].Date != "2026-03-31" {
		t.Fatalf("events %+v", ev)
	}
	for _, e := range ev {
		if e.State != "charged" {
			t.Errorf("%s should be charged", e.Date)
		}
	}
	if sumEvents(ev) != 285 {
		t.Errorf("sum %v", sumEvents(ev))
	}
	if len(data.eventsBetween(d("2027-01-01"), d("2027-12-31"), "active")) != 0 {
		t.Error("events after the last installment")
	}
	sv := data.subView(&data.Subs[0], today())
	if sv.Status != StatusCompleted || sv.Next.State != "completed" {
		t.Errorf("view %+v", sv.Next)
	}
	// a completed plan is not remembered by the reminders
	data.Settings.PushEnabled = true
	if l := data.dueReminders(today()); len(l) != 0 {
		t.Errorf("reminders for a completed plan: %v", l)
	}
}

func TestValidateInstallments(t *testing.T) {
	data := defaultData()
	mk := func(n int, price *float64) *Sub {
		s := instSub("2026-01-15", n)
		s.CategoryID = data.Categories[0].ID
		s.Method = MethodCard
		s.Frequency = FreqWeekly
		s.ManualNext = true
		s.PurchasePrice = price
		return s
	}
	if err := validateSub(data, mk(0, nil)); err == nil {
		t.Error("zero installments accepted")
	}
	if err := validateSub(data, mk(361, nil)); err == nil {
		t.Error("361 installments accepted")
	}
	neg := -5.0
	if err := validateSub(data, mk(12, &neg)); err == nil {
		t.Error("negative price accepted")
	}
	zero := 0.0
	s := mk(12, &zero)
	if err := validateSub(data, s); err != nil {
		t.Fatal(err)
	}
	if s.PurchasePrice != nil || s.Frequency != FreqMonthly || s.ManualNext {
		t.Errorf("not normalised: %+v", s)
	}
	z := instSub("2026-01-15", 12)
	z.CategoryID, z.Method = data.Categories[0].ID, MethodCard
	*z.Amount = 0
	if err := validateSub(data, z); err == nil {
		t.Error("zero amount accepted")
	}
	f := &Sub{Company: "x", CategoryID: data.Categories[0].ID, AmountType: TypeFixed, Amount: new(float64), Frequency: FreqMonthly, StartDate: "2026-01-01", Method: MethodCard, Installments: 5}
	if err := validateSub(data, f); err != nil || f.Installments != 0 {
		t.Errorf("fixed sub keeps installments: %v %+v", err, f)
	}
}
