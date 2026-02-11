package portal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/verifier"
)

// Session tracks an authenticated captive portal session.
type Session struct {
	SessionID   string
	UserDID     string
	Username    string
	StartTime   time.Time
	ExpiresAt   time.Time
	BytesIn     int64
	BytesOut    int64
	MaxBandwidth int // Mbps
}

// Server implements a captive portal for internet access sharing.
type Server struct {
	store      *store.Store
	accounting *accounting.Collector
	verifier   *verifier.Verifier
	listenAddr string

	sessions map[string]*Session
	mu       sync.RWMutex
	server   *http.Server
}

// NewServer creates a new captive portal server.
func NewServer(s *store.Store, ac *accounting.Collector, v *verifier.Verifier, listenAddr string) *Server {
	return &Server{
		store:      s,
		accounting: ac,
		verifier:   v,
		listenAddr: listenAddr,
		sessions:   make(map[string]*Session),
	}
}

// Start begins listening for captive portal requests.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleLanding)
	mux.HandleFunc("/auth", s.handleAuth)
	mux.HandleFunc("/status", s.handleStatus)

	s.server = &http.Server{
		Addr:         s.listenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("[portal] captive portal listening on %s", s.listenAddr)
	go func() {
		<-ctx.Done()
		s.server.Shutdown(context.Background())
	}()

	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("portal server error: %w", err)
	}
	return nil
}

func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>SoHoLINK Network Access</title></head>
<body>
<h1>SoHoLINK Network Access</h1>
<p>Authenticate to access the network.</p>
<form method="POST" action="/auth">
  <label>Username: <input name="username" type="text"></label><br>
  <label>Credential: <input name="credential" type="password"></label><br>
  <button type="submit">Connect</button>
</form>
</body>
</html>`)
}

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	credential := r.FormValue("credential")

	if username == "" || credential == "" {
		http.Error(w, "Missing credentials", http.StatusBadRequest)
		return
	}

	// Verify credential using existing verifier
	result, err := s.verifier.Verify(context.Background(), username, credential)
	if err != nil || !result.Allowed {
		reason := "unknown"
		if result != nil {
			reason = result.Reason
		}
		s.accounting.Record(&accounting.AccountingEvent{
			Timestamp: time.Now(),
			EventType: "portal_auth_failure",
			Username:  username,
			ClientIP:  r.RemoteAddr,
			Reason:    reason,
		})
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Create session
	session := &Session{
		SessionID: fmt.Sprintf("portal_%d", time.Now().UnixNano()),
		UserDID:   result.DID,
		Username:  username,
		StartTime: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	s.mu.Lock()
	s.sessions[session.SessionID] = session
	s.mu.Unlock()

	s.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "portal_session_started",
		UserDID:   result.DID,
		Username:  username,
		SessionID: session.SessionID,
		ClientIP:  r.RemoteAddr,
	})

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "soholink_session",
		Value:    session.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
	})

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Connected</title></head>
<body>
<h1>Connected!</h1>
<p>Welcome, %s. You are now connected to the network.</p>
<p>Session expires: %s</p>
</body>
</html>`, username, session.ExpiresAt.Format(time.RFC3339))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("soholink_session")
	if err != nil {
		http.Error(w, "No active session", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	session, ok := s.sessions[cookie.Value]
	s.mu.RUnlock()

	if !ok || time.Now().After(session.ExpiresAt) {
		http.Error(w, "Session expired", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"session_id":"%s","username":"%s","expires":"%s","bytes_in":%d,"bytes_out":%d}`,
		session.SessionID, session.Username, session.ExpiresAt.Format(time.RFC3339),
		session.BytesIn, session.BytesOut)
}

// ActiveSessions returns the count of active sessions.
func (s *Server) ActiveSessions() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	now := time.Now()
	for _, sess := range s.sessions {
		if now.Before(sess.ExpiresAt) {
			count++
		}
	}
	return count
}
