package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func missingData(repeatDays, repeatMax int) *Data {
	d := defaultData()
	d.Settings.MissingRepeatDays = repeatDays
	d.Settings.MissingRepeatMax = repeatMax
	d.Subs = []Sub{{ID: 1, Company: "Vodafone", AmountType: TypeVariable, DueDate: "2026-10-10", Status: StatusActive, RemindDays: 0, RemindPush: true}}
	return d
}

func TestMissingBillRepeats(t *testing.T) {
	d := missingData(3, 3)
	// due 10 Oct: reminders on 11, 14 and 17 Oct, then no more
	cases := []struct {
		day  string
		want int
	}{{"2026-10-10", 0}, {"2026-10-11", 1}, {"2026-10-13", 1}, {"2026-10-14", 1}, {"2026-10-17", 1}, {"2026-10-30", 1}}
	for _, c := range cases {
		d.Sent = nil
		if got := len(d.dueReminders(d2(c.day))); got != c.want {
			t.Errorf("%s: got %d want %d", c.day, got, c.want)
		}
	}
	// each step has its own key, and a sent step is not repeated the same day
	seen := map[string]bool{}
	for _, day := range []string{"2026-10-11", "2026-10-14", "2026-10-17"} {
		l := d.dueReminders(d2(day))
		if len(l) != 1 || seen[l[0].key] {
			t.Fatalf("%s: unexpected %v", day, l)
		}
		seen[l[0].key] = true
		d.Sent = append(d.Sent, l[0].key)
		if again := d.dueReminders(d2(day)); len(again) != 0 {
			t.Errorf("%s: repeated after sending", day)
		}
	}
	// after the maximum there are no more reminders
	if l := d.dueReminders(d2("2026-10-25")); len(l) != 0 {
		t.Errorf("reminders after maximum: %v", l)
	}
}

func TestMissingBillFirstKeyIsCompatible(t *testing.T) {
	d := missingData(3, 3)
	d.Sent = []string{"m:1:2026-10-10"} // sent by an older version
	if l := d.dueReminders(d2("2026-10-11")); len(l) != 0 {
		t.Errorf("first reminder sent again: %v", l)
	}
}

func TestMissingBillDisabledAndDefaults(t *testing.T) {
	d := missingData(0, 0) // zero means defaults 3 and 3
	if d.Settings.RepeatDays() != 3 || d.Settings.RepeatMax() != 3 {
		t.Fatal("defaults")
	}
	d.Settings.MissingBillReminder = false
	if l := d.dueReminders(d2("2026-10-11")); len(l) != 0 {
		t.Errorf("disabled but got %v", l)
	}
}

func TestMissingBillMaxOne(t *testing.T) {
	d := missingData(1, 1)
	l := d.dueReminders(d2("2026-10-11"))
	if len(l) != 1 {
		t.Fatal("first")
	}
	d.Sent = append(d.Sent, l[0].key)
	if l := d.dueReminders(d2("2026-10-12")); len(l) != 0 {
		t.Errorf("second reminder with max 1: %v", l)
	}
}

func d2(s string) time.Time { return d(s) }

// A reminder must stay pending when no channel could deliver it.
func TestCheckKeepsReminderPendingWithoutChannel(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(`{"status":1}`))
	}))
	defer srv.Close()
	t.Setenv("PUSHOVER_URL", srv.URL)

	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Update(func(d *Data) error {
		d.Settings.PushEnabled = true
		d.Settings.PushoverUser, d.Settings.PushoverToken = "u", "t"
		d.Settings.SendHour = 0
		d.Subs = []Sub{{ID: 1, Company: "Netflix", AmountType: TypeFixed, Frequency: FreqMonthly, StartDate: "2026-10-30", Status: StatusActive, RemindDays: 7, RemindPush: false}}
		return nil
	})
	n := NewNotifier(st)
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	n.Check(now)
	st.View(func(d *Data) {
		if len(d.Sent) != 0 || hits != 0 {
			t.Errorf("sent=%v hits=%d", d.Sent, hits)
		}
	})
	// once push is switched on for the subscription, the reminder is delivered
	_ = st.Update(func(d *Data) error { d.Subs[0].RemindPush = true; return nil })
	n.Check(now)
	st.View(func(d *Data) {
		if len(d.Sent) != 1 || hits != 1 {
			t.Errorf("sent=%v hits=%d", d.Sent, hits)
		}
	})
}
