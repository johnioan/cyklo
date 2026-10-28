package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Frequencies, amount types, methods and statuses (values stored in the data file).
const (
	FreqWeekly     = "weekly"
	FreqMonthly    = "monthly"
	FreqQuarterly  = "quarterly"
	FreqSemiannual = "semiannual"
	FreqAnnual     = "annual"

	TypeFixed    = "fixed"
	TypeVariable = "variable"

	MethodCard     = "card"
	MethodPayPal   = "paypal"
	MethodApplePay = "applepay"

	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCancelled = "cancelled"
)

type Settings struct {
	Lang                string `json:"lang"`     // "el" or "en"
	Currency            string `json:"currency"` // EUR, USD, GBP, CHF
	RemindDays          int    `json:"remindDays"`
	SendHour            int    `json:"sendHour"`
	MissingBillReminder bool   `json:"missingBillReminder"`
	MissingRepeatDays   int    `json:"missingRepeatDays"` // days between repeated "bill missing" reminders (0 = default)
	MissingRepeatMax    int    `json:"missingRepeatMax"`  // maximum number of "bill missing" reminders (0 = default)
	EmailEnabled        bool   `json:"emailEnabled"`
	PushEnabled         bool   `json:"pushEnabled"`
	PushoverUser        string `json:"pushoverUser"`
	PushoverToken       string `json:"pushoverToken"`
	OnboardingDone      bool   `json:"onboardingDone"`
	AuthMode            string `json:"authMode"` // "off" or "dsm"
	DefaultCats         bool   `json:"defaultCats"`
}

// RepeatDays returns the interval of the repeated "bill missing" reminders.
func (s Settings) RepeatDays() int {
	if s.MissingRepeatDays <= 0 {
		return 3
	}
	return s.MissingRepeatDays
}

// RepeatMax returns the maximum number of "bill missing" reminders per bill.
func (s Settings) RepeatMax() int {
	if s.MissingRepeatMax <= 0 {
		return 3
	}
	return s.MissingRepeatMax
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Card struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Sub struct {
	ID          int      `json:"id"`
	Company     string   `json:"company"`
	Service     string   `json:"service"`
	CategoryID  int      `json:"categoryId"`
	AmountType  string   `json:"amountType"`
	Amount      *float64 `json:"amount"`
	Frequency   string   `json:"frequency"`
	StartDate   string   `json:"startDate"`
	ManualNext  bool     `json:"manualNext"`
	ManualDate  string   `json:"manualDate"`
	DueDate     string   `json:"dueDate"` // variable amounts only: date of the bill that is expected next
	Method      string   `json:"method"`
	CardID      int      `json:"cardId"`
	Status      string   `json:"status"`
	RemindDays  int      `json:"remindDays"` // 0 = no reminder
	RemindEmail bool     `json:"remindEmail"`
	RemindPush  bool     `json:"remindPush"`
	Notes       string   `json:"notes"`
}

type Payment struct {
	ID     int      `json:"id"`
	SubID  int      `json:"subId"`
	Date   string   `json:"date"`
	Amount *float64 `json:"amount"`
}

// Data is the whole content of the data file.
type Data struct {
	Settings   Settings   `json:"settings"`
	Categories []Category `json:"categories"`
	Cards      []Card     `json:"cards"`
	Methods    []Category `json:"methods"` // user defined payment methods (sub.Method = "c<id>")
	Subs       []Sub      `json:"subs"`
	Payments   []Payment  `json:"payments"`
	NextID     int        `json:"nextId"`
	Sent       []string   `json:"sent"`    // keys of notifications already sent
	LastRun    string     `json:"lastRun"` // reserved
}

var greekCats = []string{"Gaming", "Παραγωγικότητα", "Streaming", "Ενέργεια και λογαριασμοί", "Τηλεπικοινωνίες", "Άλλο"}
var englishCats = []string{"Gaming", "Productivity", "Streaming", "Energy and utilities", "Telecommunications", "Other"}

func defaultData() *Data {
	d := &Data{
		Settings: Settings{
			Lang: "el", Currency: "EUR", RemindDays: 7, SendHour: 9,
			MissingBillReminder: true, MissingRepeatDays: 3, MissingRepeatMax: 3, EmailEnabled: false, PushEnabled: false,
			AuthMode: "off", DefaultCats: true,
		},
		NextID: 1,
	}
	for _, n := range greekCats {
		d.Categories = append(d.Categories, Category{ID: d.newID(), Name: n})
	}
	return d
}

func (d *Data) newID() int {
	id := d.NextID
	d.NextID++
	return id
}

// Store keeps the data in memory and persists it as JSON.
type Store struct {
	mu   sync.RWMutex
	path string
	d    *Data
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "data.json")}
	b, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		d := &Data{}
		if err := json.Unmarshal(b, d); err != nil {
			return nil, errors.New("data.json is not valid: " + err.Error())
		}
		s.d = d
	case os.IsNotExist(err):
		s.d = defaultData()
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	// keep one previous copy
	if old, err := os.ReadFile(s.path); err == nil {
		_ = os.WriteFile(s.path+".bak", old, 0o600)
	}
	return os.Rename(tmp, s.path)
}

// View runs fn with a read lock.
func (s *Store) View(fn func(d *Data)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.d)
}

// Update runs fn with a write lock and persists when fn returns nil.
func (s *Store) Update(fn func(d *Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// work on a copy so that a failed change leaves the data untouched
	b, _ := json.Marshal(s.d)
	cp := &Data{}
	_ = json.Unmarshal(b, cp)
	if err := fn(cp); err != nil {
		return err
	}
	s.d = cp
	return s.saveLocked()
}

func (s *Store) Replace(d *Data) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d = d
	return s.saveLocked()
}

func (s *Store) Snapshot() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.MarshalIndent(s.d, "", "  ")
	return b
}

// nowFunc can be replaced in tests.
var nowFunc = time.Now
