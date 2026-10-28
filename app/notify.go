package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Notifier struct {
	st     *Store
	client *http.Client
}

func NewNotifier(st *Store) *Notifier {
	return &Notifier{st: st, client: &http.Client{Timeout: 15 * time.Second}}
}

// ChannelResult is the outcome of sending through one channel.
type ChannelResult struct {
	Enabled bool   `json:"enabled"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

type TestResult struct {
	Email ChannelResult `json:"email"`
	Push  ChannelResult `json:"push"`
}

func (n *Notifier) sendPushover(user, token, title, msg string) error {
	if user == "" || token == "" {
		return errors.New("missing keys")
	}
	endpoint := os.Getenv("PUSHOVER_URL")
	if endpoint == "" {
		endpoint = "https://api.pushover.net/1/messages.json"
	}
	form := url.Values{"token": {token}, "user": {user}, "message": {msg}, "title": {title}}
	resp, err := n.client.PostForm(endpoint, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var out struct {
		Status int      `json:"status"`
		Errors []string `json:"errors"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode != 200 || out.Status != 1 {
		if len(out.Errors) > 0 {
			return errors.New(strings.Join(out.Errors, "; "))
		}
		return fmt.Errorf("pushover returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// sendDSM asks DSM to deliver a notification (DSM sends the email with its own settings).
//
// Unofficial: the tag and variable below come from community reports (DSM 7.2.1), not from Synology documentation.
// emailSupported stays false until a working email delivery method exists.
const emailSupported = false

const dsmTag, dsmVar = "DSMSupportFormCustomMessage", "%CUSTOM_MSG%"

func (n *Notifier) sendDSM(msg string) error {
	bin := os.Getenv("CYKLO_SYNONOTIFY")
	if bin == "" {
		bin = "/usr/syno/bin/synonotify"
	}
	vars, _ := json.Marshal(map[string]string{dsmVar: msg})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, dsmTag, string(vars)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (n *Notifier) deliver(s Settings, email, push bool, msg string) (emailErr, pushErr error, triedEmail, triedPush bool) {
	title := tr(s.Lang, "n.title")
	if email && s.EmailEnabled && emailSupported {
		triedEmail = true
		emailErr = n.sendDSM(msg)
	}
	if push && s.PushEnabled {
		triedPush = true
		pushErr = n.sendPushover(s.PushoverUser, s.PushoverToken, title, msg)
	}
	return
}

// SendTest sends a test message through every enabled channel.
func (n *Notifier) SendTest() TestResult {
	var s Settings
	n.st.View(func(d *Data) { s = d.Settings })
	var res TestResult
	msg := tr(s.Lang, "n.test")
	eErr, pErr, tE, tP := n.deliver(s, true, true, msg)
	res.Email = ChannelResult{Enabled: s.EmailEnabled, OK: tE && eErr == nil}
	res.Push = ChannelResult{Enabled: s.PushEnabled, OK: tP && pErr == nil}
	if eErr != nil {
		res.Email.Error = eErr.Error()
	}
	if pErr != nil {
		res.Push.Error = pErr.Error()
	}
	return res
}

type pending struct {
	key   string
	msg   string
	email bool
	push  bool
}

// due lists the reminders that should be sent now and have not been sent before.
func (d *Data) dueReminders(t time.Time) []pending {
	var out []pending
	s := d.Settings
	sent := map[string]bool{}
	for _, k := range d.Sent {
		sent[k] = true
	}
	for i := range d.Subs {
		sub := &d.Subs[i]
		if sub.Status != StatusActive {
			continue
		}
		info := nextInfo(sub, t)
		if info.Date == "" {
			continue
		}
		dt, _ := parseDate(info.Date)
		title := sub.Company
		if sub.Service != "" {
			title += " · " + sub.Service
		}
		amount := ""
		if sub.Amount != nil {
			amount = trf(s.Lang, "n.amount", money(s.Lang, s.Currency, *sub.Amount))
		}
		if info.State == "missing" {
			if !s.MissingBillReminder {
				continue
			}
			// reminder number n is due on day 1 + (n-1)*interval after the due date;
			// when several are overdue (e.g. the app was off) only the latest one is sent
			n := (-info.Days-1)/s.RepeatDays() + 1
			if n > s.RepeatMax() {
				n = s.RepeatMax()
			}
			key := fmt.Sprintf("m:%d:%s", sub.ID, info.Date)
			if n > 1 {
				key += fmt.Sprintf(":%d", n)
			}
			if !sent[key] {
				out = append(out, pending{key, trf(s.Lang, "n.missing", title, longDate(s.Lang, dt)), sub.RemindEmail, sub.RemindPush})
			}
			continue
		}
		if sub.RemindDays <= 0 || info.Days < 0 || info.Days > sub.RemindDays {
			continue
		}
		key := fmt.Sprintf("r:%d:%s", sub.ID, info.Date)
		if sent[key] {
			continue
		}
		var msg string
		switch {
		case info.State == "awaiting" && info.Days == 0:
			msg = trf(s.Lang, "n.awaitingToday", title)
		case info.State == "awaiting":
			msg = trf(s.Lang, "n.awaiting", info.Days, longDate(s.Lang, dt), title)
		case info.Days == 0:
			msg = trf(s.Lang, "n.today", title, amount)
		default:
			msg = trf(s.Lang, "n.in", info.Days, longDate(s.Lang, dt), title, amount)
		}
		out = append(out, pending{key, msg, sub.RemindEmail, sub.RemindPush})
	}
	return out
}

// Check sends the reminders that are due. It is safe to call often.
func (n *Notifier) Check(now time.Time) {
	var s Settings
	var list []pending
	n.st.View(func(d *Data) {
		s = d.Settings
		if now.Hour() < s.SendHour {
			return
		}
		list = d.dueReminders(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC))
	})
	if len(list) == 0 {
		return
	}
	var done []string
	for _, p := range list {
		eErr, pErr, tE, tP := n.deliver(s, p.email, p.push, p.msg)
		if eErr != nil {
			log.Printf("email reminder failed: %v", eErr)
		}
		if pErr != nil {
			log.Printf("pushover reminder failed: %v", pErr)
		}
		// mark as sent only when at least one channel delivered it; otherwise it stays pending
		if (tE && eErr == nil) || (tP && pErr == nil) {
			done = append(done, p.key)
		}
	}
	if len(done) > 0 {
		_ = n.st.Update(func(d *Data) error {
			d.Sent = append(d.Sent, done...)
			if len(d.Sent) > 1000 {
				d.Sent = d.Sent[len(d.Sent)-1000:]
			}
			return nil
		})
	}
}

func (n *Notifier) Run(ctx context.Context) {
	tk := time.NewTicker(time.Minute)
	defer tk.Stop()
	n.Check(nowFunc())
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			n.Check(nowFunc())
		}
	}
}
