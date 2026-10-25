package main

import (
	"fmt"
	"strings"
	"time"
)

var strs = map[string]map[string]string{
	"el": {
		"weekly": "Εβδομαδιαία", "monthly": "Μηνιαία", "quarterly": "Τριμηνιαία", "semiannual": "Εξαμηνιαία", "annual": "Ετήσια",
		"card": "Πιστωτική κάρτα", "paypal": "PayPal", "applepay": "Apple Pay",
		"active": "Ενεργή", "paused": "Σε παύση", "cancelled": "Ακυρωμένη",
		"fixed": "Σταθερό ποσό", "variable": "Μεταβλητό ποσό",
		"awaiting": "Αναμονή λογαριασμού", "missing": "Λείπει ο λογαριασμός",
		"h.company": "Εταιρεία", "h.service": "Υπηρεσία", "h.category": "Κατηγορία", "h.amountType": "Είδος ποσού",
		"h.amount": "Ποσό", "h.frequency": "Συχνότητα", "h.start": "Έναρξη", "h.next": "Επόμενη πληρωμή",
		"h.method": "Τρόπος πληρωμής", "h.card": "Κάρτα", "h.status": "Κατάσταση", "h.remind": "Υπενθύμιση",
		"h.notes": "Σημειώσεις", "h.group": "Ομάδα", "h.subs": "Συνδρομές", "h.pct": "Ποσοστό", "h.total": "Σύνολο",
		"remind.0": "Χωρίς υπενθύμιση", "remind.n": "%d ημέρες πριν", "remind.1": "1 ημέρα πριν",
		"sheet.subs": "Συνδρομές", "sheet.report": "Αναφορά",
		"n.title":         "Συνδρομές",
		"n.in":            "Πληρωμή σε %d ημέρες (%s): %s%s",
		"n.today":         "Πληρωμή σήμερα: %s%s",
		"n.awaiting":      "Αναμένεται λογαριασμός σε %d ημέρες (%s): %s",
		"n.awaitingToday": "Αναμένεται λογαριασμός σήμερα: %s",
		"n.missing":       "Λείπει ο λογαριασμός %s (ημερομηνία %s). Καταχώρηση ποσού στο Cyklo.",
		"n.test":          "Δοκιμαστική ειδοποίηση από το Cyklo.",
		"n.amount":        " – %s",
		"n.noChannel":     "δεν έχει ενεργοποιηθεί κανάλι",
		"n.noPushCfg":     "λείπουν τα κλειδιά Pushover",
	},
	"en": {
		"weekly": "Weekly", "monthly": "Monthly", "quarterly": "Quarterly", "semiannual": "Semi-annual", "annual": "Annual",
		"card": "Credit card", "paypal": "PayPal", "applepay": "Apple Pay",
		"active": "Active", "paused": "Paused", "cancelled": "Cancelled",
		"fixed": "Fixed amount", "variable": "Variable amount",
		"awaiting": "Awaiting bill", "missing": "Bill missing",
		"h.company": "Company", "h.service": "Service", "h.category": "Category", "h.amountType": "Amount type",
		"h.amount": "Amount", "h.frequency": "Frequency", "h.start": "Start date", "h.next": "Next payment",
		"h.method": "Payment method", "h.card": "Card", "h.status": "Status", "h.remind": "Reminder",
		"h.notes": "Notes", "h.group": "Group", "h.subs": "Subscriptions", "h.pct": "Share", "h.total": "Total",
		"remind.0": "No reminder", "remind.n": "%d days before", "remind.1": "1 day before",
		"sheet.subs": "Subscriptions", "sheet.report": "Report",
		"n.title":         "Subscriptions",
		"n.in":            "Payment in %d days (%s): %s%s",
		"n.today":         "Payment due today: %s%s",
		"n.awaiting":      "Bill expected in %d days (%s): %s",
		"n.awaitingToday": "Bill expected today: %s",
		"n.missing":       "Bill missing for %s (date %s). Enter the amount in Cyklo.",
		"n.test":          "Test notification from Cyklo.",
		"n.amount":        " – %s",
		"n.noChannel":     "no channel is enabled",
		"n.noPushCfg":     "Pushover keys are missing",
	},
}

func tr(lang, key string) string {
	if m, ok := strs[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := strs["en"][key]; ok {
		return v
	}
	return key
}

func trf(lang, key string, a ...any) string { return fmt.Sprintf(tr(lang, key), a...) }

func currencySymbol(c string) string {
	switch c {
	case "USD":
		return "$"
	case "GBP":
		return "£"
	case "CHF":
		return "CHF"
	}
	return "€"
}

// money formats an amount for messages, e.g. "€ 12,50" (el) or "€12.50" (en).
func money(lang, cur string, v float64) string {
	s := fmt.Sprintf("%.2f", v)
	sym := currencySymbol(cur)
	if lang == "el" {
		s = strings.Replace(s, ".", ",", 1)
		return sym + " " + s
	}
	return sym + s
}

func longDate(lang string, t time.Time) string {
	months := []string{"Ιαν", "Φεβ", "Μαρ", "Απρ", "Μάι", "Ιουν", "Ιουλ", "Αυγ", "Σεπ", "Οκτ", "Νοε", "Δεκ"}
	if lang == "el" {
		return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
	}
	return t.Format("2 Jan 2006")
}
