package security

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// FirewallRule rappresenta una regola del firewall
type FirewallRule struct {
	Type      RuleType
	Value     string
	Action    RuleAction
	ExpiresAt *time.Time
}

// RuleType definisce il tipo di regola
type RuleType int

const (
	RuleTypeIP RuleType = iota
	RuleTypeCIDR
	RuleTypeCountry
	RuleTypeASN
)

// RuleAction definisce l'azione della regola
type RuleAction int

const (
	RuleActionAllow RuleAction = iota
	RuleActionDeny
)

// Firewall gestisce il firewall delle richieste
type Firewall struct {
	mu              sync.RWMutex
	rules           []FirewallRule
	ipWhitelist     map[string]bool
	ipBlacklist     map[string]bool
	cidrWhitelist   []*net.IPNet
	cidrBlacklist   []*net.IPNet
	rateLimiter     *RateLimiter
	ddosProtection  *DDoSProtection
	geoRestrictions map[string]bool // country code -> allowed
}

// NewFirewall crea un nuovo firewall
func NewFirewall() *Firewall {
	return &Firewall{
		rules:           make([]FirewallRule, 0),
		ipWhitelist:     make(map[string]bool),
		ipBlacklist:     make(map[string]bool),
		cidrWhitelist:   make([]*net.IPNet, 0),
		cidrBlacklist:   make([]*net.IPNet, 0),
		rateLimiter:     NewRateLimiter(100, time.Minute), // 100 req/min default
		ddosProtection:  NewDDoSProtection(),
		geoRestrictions: make(map[string]bool),
	}
}

// AddIPWhitelist aggiunge un IP alla whitelist
func (f *Firewall) AddIPWhitelist(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	f.ipWhitelist[ip] = true
	return nil
}

// AddIPBlacklist aggiunge un IP alla blacklist
func (f *Firewall) AddIPBlacklist(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	f.ipBlacklist[ip] = true
	return nil
}

// AddCIDRWhitelist aggiunge un CIDR alla whitelist
func (f *Firewall) AddCIDRWhitelist(cidr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %s", cidr)
	}

	f.cidrWhitelist = append(f.cidrWhitelist, ipNet)
	return nil
}

// AddCIDRBlacklist aggiunge un CIDR alla blacklist
func (f *Firewall) AddCIDRBlacklist(cidr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %s", cidr)
	}

	f.cidrBlacklist = append(f.cidrBlacklist, ipNet)
	return nil
}

// RemoveIPWhitelist rimuove un IP dalla whitelist
func (f *Firewall) RemoveIPWhitelist(ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.ipWhitelist, ip)
}

// RemoveIPBlacklist rimuove un IP dalla blacklist
func (f *Firewall) RemoveIPBlacklist(ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.ipBlacklist, ip)
}

// IsIPAllowed verifica se un IP è consentito
func (f *Firewall) IsIPAllowed(ip string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Controlla blacklist IP
	if f.ipBlacklist[ip] {
		return false
	}

	// Controlla blacklist CIDR
	for _, ipNet := range f.cidrBlacklist {
		if ipNet.Contains(parsedIP) {
			return false
		}
	}

	// Se c'è una whitelist, l'IP deve essere nella whitelist
	if len(f.ipWhitelist) > 0 || len(f.cidrWhitelist) > 0 {
		// Controlla whitelist IP
		if f.ipWhitelist[ip] {
			return true
		}

		// Controlla whitelist CIDR
		for _, ipNet := range f.cidrWhitelist {
			if ipNet.Contains(parsedIP) {
				return true
			}
		}

		return false
	}

	return true
}

// AddGeoRestriction aggiunge una restrizione geografica
func (f *Firewall) AddGeoRestriction(countryCode string, allowed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.geoRestrictions[strings.ToUpper(countryCode)] = allowed
}

// IsCountryAllowed verifica se un paese è consentito
func (f *Firewall) IsCountryAllowed(countryCode string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.geoRestrictions) == 0 {
		return true
	}

	allowed, exists := f.geoRestrictions[strings.ToUpper(countryCode)]
	return exists && allowed
}

// CheckRequest verifica se una richiesta è consentita
func (f *Firewall) CheckRequest(ip, countryCode string) (allowed bool, reason string) {
	// Verifica IP
	if !f.IsIPAllowed(ip) {
		return false, "IP blocked"
	}

	// Verifica geofencing
	if !f.IsCountryAllowed(countryCode) {
		return false, "country blocked"
	}

	// Verifica rate limiting
	if !f.rateLimiter.Allow(ip) {
		return false, "rate limit exceeded"
	}

	// Verifica DDoS protection
	if !f.ddosProtection.AllowRequest(ip) {
		return false, "DDoS protection triggered"
	}

	return true, ""
}

// RateLimiter implementa rate limiting per IP
type RateLimiter struct {
	mu       sync.RWMutex
	limits   map[string]*ipLimit
	maxReqs  int
	window   time.Duration
	cleanup  *time.Ticker
	stopChan chan bool
}

type ipLimit struct {
	requests []time.Time
	blocked  bool
}

// NewRateLimiter crea un nuovo rate limiter
func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limits:   make(map[string]*ipLimit),
		maxReqs:  maxReqs,
		window:   window,
		cleanup:  time.NewTicker(time.Minute),
		stopChan: make(chan bool),
	}

	// Cleanup periodico
	go rl.cleanupLoop()

	return rl
}

// Allow verifica se una richiesta è consentita
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	limit, exists := rl.limits[ip]

	if !exists {
		limit = &ipLimit{
			requests: make([]time.Time, 0),
		}
		rl.limits[ip] = limit
	}

	// Rimuovi richieste vecchie
	cutoff := now.Add(-rl.window)
	validRequests := make([]time.Time, 0)
	for _, t := range limit.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	limit.requests = validRequests

	// Verifica limite
	if len(limit.requests) >= rl.maxReqs {
		limit.blocked = true
		return false
	}

	// Aggiungi richiesta
	limit.requests = append(limit.requests, now)
	return true
}

// cleanupLoop pulisce periodicamente i dati vecchi
func (rl *RateLimiter) cleanupLoop() {
	for {
		select {
		case <-rl.cleanup.C:
			rl.mu.Lock()
			now := time.Now()
			cutoff := now.Add(-rl.window * 2)

			for ip, limit := range rl.limits {
				if len(limit.requests) == 0 {
					delete(rl.limits, ip)
					continue
				}

				lastRequest := limit.requests[len(limit.requests)-1]
				if lastRequest.Before(cutoff) {
					delete(rl.limits, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopChan:
			rl.cleanup.Stop()
			return
		}
	}
}

// Stop ferma il rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

// DDoSProtection implementa protezione DDoS
type DDoSProtection struct {
	mu              sync.RWMutex
	requestCounts   map[string]*ddosStats
	threshold       int
	windowSize      time.Duration
	blockDuration   time.Duration
	cleanup         *time.Ticker
	stopChan        chan bool
}

type ddosStats struct {
	count       int
	windowStart time.Time
	blocked     bool
	blockedUntil time.Time
}

// NewDDoSProtection crea una nuova protezione DDoS
func NewDDoSProtection() *DDoSProtection {
	ddos := &DDoSProtection{
		requestCounts: make(map[string]*ddosStats),
		threshold:     1000,          // 1000 richieste
		windowSize:    time.Second,   // per secondo
		blockDuration: 5 * time.Minute, // blocco per 5 minuti
		cleanup:       time.NewTicker(time.Minute),
		stopChan:      make(chan bool),
	}

	go ddos.cleanupLoop()

	return ddos
}

// AllowRequest verifica se una richiesta è consentita
func (ddos *DDoSProtection) AllowRequest(ip string) bool {
	ddos.mu.Lock()
	defer ddos.mu.Unlock()

	now := time.Now()
	stats, exists := ddos.requestCounts[ip]

	if !exists {
		stats = &ddosStats{
			count:       1,
			windowStart: now,
		}
		ddos.requestCounts[ip] = stats
		return true
	}

	// Verifica se l'IP è bloccato
	if stats.blocked && now.Before(stats.blockedUntil) {
		return false
	}

	// Reset se la finestra è scaduta
	if now.Sub(stats.windowStart) > ddos.windowSize {
		stats.count = 1
		stats.windowStart = now
		stats.blocked = false
		return true
	}

	// Incrementa counter
	stats.count++

	// Verifica threshold
	if stats.count > ddos.threshold {
		stats.blocked = true
		stats.blockedUntil = now.Add(ddos.blockDuration)
		return false
	}

	return true
}

// cleanupLoop pulisce periodicamente i dati vecchi
func (ddos *DDoSProtection) cleanupLoop() {
	for {
		select {
		case <-ddos.cleanup.C:
			ddos.mu.Lock()
			now := time.Now()

			for ip, stats := range ddos.requestCounts {
				// Rimuovi se la finestra è scaduta e non è bloccato
				if now.Sub(stats.windowStart) > ddos.windowSize*2 && !stats.blocked {
					delete(ddos.requestCounts, ip)
				}

				// Rimuovi se il blocco è scaduto
				if stats.blocked && now.After(stats.blockedUntil) {
					delete(ddos.requestCounts, ip)
				}
			}
			ddos.mu.Unlock()
		case <-ddos.stopChan:
			ddos.cleanup.Stop()
			return
		}
	}
}

// Stop ferma la protezione DDoS
func (ddos *DDoSProtection) Stop() {
	close(ddos.stopChan)
}

// GetBlockedIPs ritorna gli IP attualmente bloccati
func (ddos *DDoSProtection) GetBlockedIPs() []string {
	ddos.mu.RLock()
	defer ddos.mu.RUnlock()

	now := time.Now()
	blocked := make([]string, 0)

	for ip, stats := range ddos.requestCounts {
		if stats.blocked && now.Before(stats.blockedUntil) {
			blocked = append(blocked, ip)
		}
	}

	return blocked
}
