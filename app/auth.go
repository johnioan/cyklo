package main

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// dsmUser asks DSM who is logged in, forwarding the browser cookies of the request.
// DSM documents authenticate.cgi as returning the user name when the session is valid.
func dsmUser(r *http.Request) (string, error) {
	endpoint := os.Getenv("CYKLO_DSM_AUTH")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:5000/webman/authenticate.cgi"
	}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	if c := r.Header.Get("Cookie"); c != "" {
		req.Header.Set("Cookie", c)
	}
	if t := r.Header.Get("X-SYNO-TOKEN"); t != "" {
		req.Header.Set("X-SYNO-TOKEN", t)
	}
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	user := strings.TrimSpace(string(b))
	if resp.StatusCode != 200 || user == "" || strings.ContainsAny(user, "<>\n") {
		return "", nil
	}
	return user, nil
}

func (a *API) authCheck(w http.ResponseWriter, r *http.Request) {
	var mode string
	a.st.View(func(d *Data) { mode = d.Settings.AuthMode })
	out := map[string]any{"mode": mode, "ok": true}
	if mode == "dsm" {
		u, err := dsmUser(r)
		out["ok"] = err == nil && u != ""
		out["user"] = u
		if err != nil {
			out["error"] = err.Error()
		}
	}
	writeJSON(w, 200, out)
}

// requireAuth protects the data API when the DSM login mode is on.
func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/auth/check" {
			var mode string
			a.st.View(func(d *Data) { mode = d.Settings.AuthMode })
			if mode == "dsm" {
				u, err := dsmUser(r)
				if err != nil || u == "" {
					writeErr(w, 401, "login required")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
