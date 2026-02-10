package mobile

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// AuthMethod rappresenta un metodo di autenticazione
type AuthMethod string

const (
	AuthMethodPassword    AuthMethod = "password"
	AuthMethodBiometric   AuthMethod = "biometric"
	AuthMethodOAuth2      AuthMethod = "oauth2"
	AuthMethodDeviceToken AuthMethod = "device_token"
)

// BiometricType rappresenta il tipo di autenticazione biometrica
type BiometricType string

const (
	BiometricFaceID      BiometricType = "face_id"
	BiometricTouchID     BiometricType = "touch_id"
	BiometricFingerprint BiometricType = "fingerprint"
	BiometricIris        BiometricType = "iris"
)

// TokenType rappresenta il tipo di token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeDevice  TokenType = "device"
)

// OAuth2PKCERequest rappresenta una richiesta OAuth2 con PKCE
type OAuth2PKCERequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"` // "S256" or "plain"
	State               string `json:"state"`
	Scope               string `json:"scope"`
}

// OAuth2TokenRequest rappresenta una richiesta di token OAuth2
type OAuth2TokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// OAuth2TokenResponse rappresenta una risposta con token OAuth2
type OAuth2TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// BiometricAuthRequest rappresenta una richiesta di autenticazione biometrica
type BiometricAuthRequest struct {
	UserID        string        `json:"user_id"`
	DeviceID      string        `json:"device_id"`
	BiometricType BiometricType `json:"biometric_type"`
	Challenge     string        `json:"challenge"`
	Signature     string        `json:"signature"`
}

// Device rappresenta un dispositivo registrato
type Device struct {
	ID                string        `json:"id"`
	UserID            string        `json:"user_id"`
	Name              string        `json:"name"`
	Platform          Platform      `json:"platform"`
	OSVersion         string        `json:"os_version"`
	AppVersion        string        `json:"app_version"`
	DeviceToken       string        `json:"device_token"`
	BiometricEnabled  bool          `json:"biometric_enabled"`
	BiometricType     BiometricType `json:"biometric_type,omitempty"`
	PublicKey         string        `json:"public_key,omitempty"` // Per biometric auth
	LastLoginAt       time.Time     `json:"last_login_at"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	Trusted           bool          `json:"trusted"`
	NotificationToken string        `json:"notification_token,omitempty"`
}

// RefreshToken rappresenta un refresh token
type RefreshToken struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	Family    string    `json:"family"` // Per token rotation
	Version   int       `json:"version"` // Per token rotation
}

// AccessToken rappresenta un access token
type AccessToken struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Scope     []string  `json:"scope"`
}

// PKCESession rappresenta una sessione PKCE
type PKCESession struct {
	Code                string    `json:"code"`
	CodeChallenge       string    `json:"code_challenge"`
	CodeChallengeMethod string    `json:"code_challenge_method"`
	ClientID            string    `json:"client_id"`
	RedirectURI         string    `json:"redirect_uri"`
	State               string    `json:"state"`
	UserID              string    `json:"user_id,omitempty"`
	ExpiresAt           time.Time `json:"expires_at"`
	Used                bool      `json:"used"`
}

// MobileAuthService gestisce l'autenticazione mobile
type MobileAuthService struct {
	devices        map[string]*Device       // key: device_id
	refreshTokens  map[string]*RefreshToken // key: token
	accessTokens   map[string]*AccessToken  // key: token
	pkceSessions   map[string]*PKCESession  // key: code
	userDevices    map[string][]string      // key: user_id, value: device_ids
	tokenFamilies  map[string][]*RefreshToken // key: family_id
	mu             sync.RWMutex
	accessTTL      time.Duration
	refreshTTL     time.Duration
	maxDevices     int
}

// NewMobileAuthService crea un nuovo servizio di autenticazione mobile
func NewMobileAuthService() *MobileAuthService {
	return &MobileAuthService{
		devices:       make(map[string]*Device),
		refreshTokens: make(map[string]*RefreshToken),
		accessTokens:  make(map[string]*AccessToken),
		pkceSessions:  make(map[string]*PKCESession),
		userDevices:   make(map[string][]string),
		tokenFamilies: make(map[string][]*RefreshToken),
		accessTTL:     15 * time.Minute,
		refreshTTL:    30 * 24 * time.Hour, // 30 giorni
		maxDevices:    10,
	}
}

// InitiatePKCEFlow inizia un flusso OAuth2 PKCE
func (mas *MobileAuthService) InitiatePKCEFlow(ctx context.Context, req *OAuth2PKCERequest) (*PKCESession, error) {
	if req.CodeChallengeMethod != "S256" && req.CodeChallengeMethod != "plain" {
		return nil, errors.New("invalid code_challenge_method")
	}

	code, err := generateRandomToken(32)
	if err != nil {
		return nil, err
	}

	session := &PKCESession{
		Code:                code,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ClientID:            req.ClientID,
		RedirectURI:         req.RedirectURI,
		State:               req.State,
		ExpiresAt:           time.Now().Add(10 * time.Minute),
		Used:                false,
	}

	mas.mu.Lock()
	mas.pkceSessions[code] = session
	mas.mu.Unlock()

	return session, nil
}

// ExchangeCodeForToken scambia un authorization code per un token
func (mas *MobileAuthService) ExchangeCodeForToken(ctx context.Context, req *OAuth2TokenRequest, userID, deviceID string) (*OAuth2TokenResponse, error) {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	session, exists := mas.pkceSessions[req.Code]
	if !exists || session.Used || time.Now().After(session.ExpiresAt) {
		return nil, errors.New("invalid or expired authorization code")
	}

	// Verifica il code verifier
	if !mas.verifyCodeChallenge(req.CodeVerifier, session.CodeChallenge, session.CodeChallengeMethod) {
		return nil, errors.New("invalid code verifier")
	}

	session.Used = true
	session.UserID = userID

	// Genera i token
	accessToken, err := mas.createAccessToken(userID, deviceID, []string{"read", "write"})
	if err != nil {
		return nil, err
	}

	refreshToken, err := mas.createRefreshToken(userID, deviceID)
	if err != nil {
		return nil, err
	}

	return &OAuth2TokenResponse{
		AccessToken:  accessToken.Token,
		TokenType:    "Bearer",
		ExpiresIn:    int64(mas.accessTTL.Seconds()),
		RefreshToken: refreshToken.Token,
		Scope:        "read write",
	}, nil
}

// RefreshAccessToken rinnova un access token usando un refresh token
func (mas *MobileAuthService) RefreshAccessToken(ctx context.Context, refreshTokenStr string) (*OAuth2TokenResponse, error) {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	refreshToken, exists := mas.refreshTokens[refreshTokenStr]
	if !exists {
		return nil, errors.New("invalid refresh token")
	}

	if refreshToken.Revoked || time.Now().After(refreshToken.ExpiresAt) {
		return nil, errors.New("refresh token expired or revoked")
	}

	// Token rotation: revoca il vecchio refresh token
	refreshToken.Revoked = true
	now := time.Now()
	refreshToken.RevokedAt = &now

	// Verifica se ci sono stati tentativi di riuso (possibile attacco)
	if mas.detectTokenReuse(refreshToken) {
		// Revoca tutti i token della famiglia
		mas.revokeTokenFamily(refreshToken.Family)
		return nil, errors.New("token reuse detected - all tokens revoked")
	}

	// Crea nuovi token
	accessToken, err := mas.createAccessToken(refreshToken.UserID, refreshToken.DeviceID, []string{"read", "write"})
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := mas.createRefreshTokenInFamily(refreshToken.UserID, refreshToken.DeviceID, refreshToken.Family, refreshToken.Version+1)
	if err != nil {
		return nil, err
	}

	return &OAuth2TokenResponse{
		AccessToken:  accessToken.Token,
		TokenType:    "Bearer",
		ExpiresIn:    int64(mas.accessTTL.Seconds()),
		RefreshToken: newRefreshToken.Token,
		Scope:        "read write",
	}, nil
}

// AuthenticateBiometric autentica con biometria
func (mas *MobileAuthService) AuthenticateBiometric(ctx context.Context, req *BiometricAuthRequest) (*OAuth2TokenResponse, error) {
	mas.mu.RLock()
	device, exists := mas.devices[req.DeviceID]
	mas.mu.RUnlock()

	if !exists || device.UserID != req.UserID {
		return nil, errors.New("device not found or unauthorized")
	}

	if !device.BiometricEnabled {
		return nil, errors.New("biometric authentication not enabled for this device")
	}

	// Verifica la firma biometrica
	if !mas.verifyBiometricSignature(req, device) {
		return nil, errors.New("invalid biometric signature")
	}

	// Genera i token
	mas.mu.Lock()
	accessToken, err := mas.createAccessToken(req.UserID, req.DeviceID, []string{"read", "write"})
	if err != nil {
		mas.mu.Unlock()
		return nil, err
	}

	refreshToken, err := mas.createRefreshToken(req.UserID, req.DeviceID)
	if err != nil {
		mas.mu.Unlock()
		return nil, err
	}
	mas.mu.Unlock()

	// Aggiorna ultimo login
	device.LastLoginAt = time.Now()

	return &OAuth2TokenResponse{
		AccessToken:  accessToken.Token,
		TokenType:    "Bearer",
		ExpiresIn:    int64(mas.accessTTL.Seconds()),
		RefreshToken: refreshToken.Token,
		Scope:        "read write",
	}, nil
}

// RegisterDevice registra un nuovo dispositivo
func (mas *MobileAuthService) RegisterDevice(ctx context.Context, device *Device) error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	// Verifica limite dispositivi per utente
	if userDevices, exists := mas.userDevices[device.UserID]; exists {
		if len(userDevices) >= mas.maxDevices {
			return fmt.Errorf("maximum number of devices (%d) reached", mas.maxDevices)
		}
	}

	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()
	device.LastLoginAt = time.Now()

	mas.devices[device.ID] = device

	// Aggiungi all'indice utente
	if _, exists := mas.userDevices[device.UserID]; !exists {
		mas.userDevices[device.UserID] = make([]string, 0)
	}
	mas.userDevices[device.UserID] = append(mas.userDevices[device.UserID], device.ID)

	return nil
}

// EnableBiometric abilita l'autenticazione biometrica per un dispositivo
func (mas *MobileAuthService) EnableBiometric(ctx context.Context, deviceID string, biometricType BiometricType, publicKey string) error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	device, exists := mas.devices[deviceID]
	if !exists {
		return errors.New("device not found")
	}

	device.BiometricEnabled = true
	device.BiometricType = biometricType
	device.PublicKey = publicKey
	device.UpdatedAt = time.Now()

	return nil
}

// DisableBiometric disabilita l'autenticazione biometrica
func (mas *MobileAuthService) DisableBiometric(ctx context.Context, deviceID string) error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	device, exists := mas.devices[deviceID]
	if !exists {
		return errors.New("device not found")
	}

	device.BiometricEnabled = false
	device.BiometricType = ""
	device.PublicKey = ""
	device.UpdatedAt = time.Now()

	return nil
}

// GetUserDevices ottiene tutti i dispositivi di un utente
func (mas *MobileAuthService) GetUserDevices(userID string) []*Device {
	mas.mu.RLock()
	defer mas.mu.RUnlock()

	deviceIDs, exists := mas.userDevices[userID]
	if !exists {
		return []*Device{}
	}

	devices := make([]*Device, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if device, exists := mas.devices[deviceID]; exists {
			devices = append(devices, device)
		}
	}

	return devices
}

// RevokeDevice revoca un dispositivo
func (mas *MobileAuthService) RevokeDevice(ctx context.Context, deviceID string) error {
	mas.mu.Lock()
	defer mas.mu.Unlock()

	device, exists := mas.devices[deviceID]
	if !exists {
		return errors.New("device not found")
	}

	// Revoca tutti i refresh token del dispositivo
	for _, rt := range mas.refreshTokens {
		if rt.DeviceID == deviceID && !rt.Revoked {
			rt.Revoked = true
			now := time.Now()
			rt.RevokedAt = &now
		}
	}

	// Rimuovi dagli indici
	if deviceIDs, exists := mas.userDevices[device.UserID]; exists {
		newDeviceIDs := make([]string, 0)
		for _, id := range deviceIDs {
			if id != deviceID {
				newDeviceIDs = append(newDeviceIDs, id)
			}
		}
		mas.userDevices[device.UserID] = newDeviceIDs
	}

	delete(mas.devices, deviceID)
	return nil
}

// ValidateAccessToken valida un access token
func (mas *MobileAuthService) ValidateAccessToken(tokenStr string) (*AccessToken, error) {
	mas.mu.RLock()
	defer mas.mu.RUnlock()

	token, exists := mas.accessTokens[tokenStr]
	if !exists {
		return nil, errors.New("invalid access token")
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, errors.New("access token expired")
	}

	return token, nil
}

// createAccessToken crea un nuovo access token
func (mas *MobileAuthService) createAccessToken(userID, deviceID string, scope []string) (*AccessToken, error) {
	tokenStr, err := generateRandomToken(32)
	if err != nil {
		return nil, err
	}

	token := &AccessToken{
		Token:     tokenStr,
		UserID:    userID,
		DeviceID:  deviceID,
		ExpiresAt: time.Now().Add(mas.accessTTL),
		CreatedAt: time.Now(),
		Scope:     scope,
	}

	mas.accessTokens[tokenStr] = token
	return token, nil
}

// createRefreshToken crea un nuovo refresh token
func (mas *MobileAuthService) createRefreshToken(userID, deviceID string) (*RefreshToken, error) {
	tokenStr, err := generateRandomToken(48)
	if err != nil {
		return nil, err
	}

	family, err := generateRandomToken(16)
	if err != nil {
		return nil, err
	}

	token := &RefreshToken{
		Token:     tokenStr,
		UserID:    userID,
		DeviceID:  deviceID,
		ExpiresAt: time.Now().Add(mas.refreshTTL),
		CreatedAt: time.Now(),
		Revoked:   false,
		Family:    family,
		Version:   1,
	}

	mas.refreshTokens[tokenStr] = token
	mas.tokenFamilies[family] = []*RefreshToken{token}

	return token, nil
}

// createRefreshTokenInFamily crea un refresh token nella stessa famiglia
func (mas *MobileAuthService) createRefreshTokenInFamily(userID, deviceID, family string, version int) (*RefreshToken, error) {
	tokenStr, err := generateRandomToken(48)
	if err != nil {
		return nil, err
	}

	token := &RefreshToken{
		Token:     tokenStr,
		UserID:    userID,
		DeviceID:  deviceID,
		ExpiresAt: time.Now().Add(mas.refreshTTL),
		CreatedAt: time.Now(),
		Revoked:   false,
		Family:    family,
		Version:   version,
	}

	mas.refreshTokens[tokenStr] = token
	mas.tokenFamilies[family] = append(mas.tokenFamilies[family], token)

	return token, nil
}

// detectTokenReuse rileva il riuso di token
func (mas *MobileAuthService) detectTokenReuse(token *RefreshToken) bool {
	// Se il token è già stato revocato e viene usato di nuovo, è un riuso
	return token.Revoked && token.RevokedAt != nil
}

// revokeTokenFamily revoca tutti i token di una famiglia
func (mas *MobileAuthService) revokeTokenFamily(family string) {
	if tokens, exists := mas.tokenFamilies[family]; exists {
		now := time.Now()
		for _, token := range tokens {
			if !token.Revoked {
				token.Revoked = true
				token.RevokedAt = &now
			}
		}
	}
}

// verifyCodeChallenge verifica il code verifier PKCE
func (mas *MobileAuthService) verifyCodeChallenge(verifier, challenge, method string) bool {
	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		return computed == challenge
	case "plain":
		return verifier == challenge
	default:
		return false
	}
}

// verifyBiometricSignature verifica la firma biometrica
func (mas *MobileAuthService) verifyBiometricSignature(req *BiometricAuthRequest, device *Device) bool {
	// In produzione, qui verificheresti la firma usando la chiave pubblica del dispositivo
	// Per ora, simuliamo una verifica semplice
	if device.PublicKey == "" {
		return false
	}

	// Verifica che la firma non sia vuota
	return req.Signature != ""
}

// generateRandomToken genera un token casuale
func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCodeChallenge genera un code challenge PKCE (utility per client)
func GenerateCodeChallenge() (verifier, challenge string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	verifier = base64.RawURLEncoding.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])

	return verifier, challenge, nil
}
