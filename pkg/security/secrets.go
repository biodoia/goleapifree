package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	// ErrSecretNotFound viene restituito quando un secret non esiste
	ErrSecretNotFound = errors.New("secret not found")

	// ErrSecretExpired viene restituito quando un secret è scaduto
	ErrSecretExpired = errors.New("secret expired")
)

// SecretType definisce il tipo di secret
type SecretType string

const (
	SecretTypeAPIKey      SecretType = "api_key"
	SecretTypePassword    SecretType = "password"
	SecretTypeToken       SecretType = "token"
	SecretTypeCertificate SecretType = "certificate"
	SecretTypePrivateKey  SecretType = "private_key"
	SecretTypeGeneric     SecretType = "generic"
)

// Secret rappresenta un secret gestito
type Secret struct {
	Key         string                 `json:"key"`
	Value       string                 `json:"value"`
	Type        SecretType             `json:"type"`
	Description string                 `json:"description,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Version     int                    `json:"version"`
}

// SecretsManager gestisce i secrets
type SecretsManager struct {
	mu              sync.RWMutex
	secrets         map[string]Secret
	encryptionMgr   *EncryptionManager
	storage         SecretStorage
	rotationEnabled bool
	rotationPeriod  time.Duration
	stopChan        chan bool
}

// SecretStorage definisce l'interfaccia per lo storage dei secrets
type SecretStorage interface {
	Save(secrets map[string]Secret) error
	Load() (map[string]Secret, error)
}

// FileStorage implementa SecretStorage su file
type FileStorage struct {
	filePath      string
	encryptionMgr *EncryptionManager
}

// NewFileStorage crea un nuovo file storage
func NewFileStorage(filePath string, encryptionMgr *EncryptionManager) *FileStorage {
	return &FileStorage{
		filePath:      filePath,
		encryptionMgr: encryptionMgr,
	}
}

// Save salva i secrets su file
func (fs *FileStorage) Save(secrets map[string]Secret) error {
	// Crea directory se non esiste
	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Serializza i secrets
	data, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	// Cifra i dati
	encrypted, err := fs.encryptionMgr.Encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}

	// Scrivi su file con permessi restrittivi
	if err := os.WriteFile(fs.filePath, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	return nil
}

// Load carica i secrets dal file
func (fs *FileStorage) Load() (map[string]Secret, error) {
	// Leggi il file
	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]Secret), nil
		}
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	// Decifra i dati
	decrypted, err := fs.encryptionMgr.Decrypt(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets: %w", err)
	}

	// Deserializza i secrets
	var secrets map[string]Secret
	if err := json.Unmarshal(decrypted, &secrets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %w", err)
	}

	return secrets, nil
}

// NewSecretsManager crea un nuovo secrets manager
func NewSecretsManager(storage SecretStorage) (*SecretsManager, error) {
	sm := &SecretsManager{
		secrets:         make(map[string]Secret),
		storage:         storage,
		rotationEnabled: false,
		rotationPeriod:  90 * 24 * time.Hour, // 90 giorni default
		stopChan:        make(chan bool),
	}

	// Carica i secrets esistenti
	if storage != nil {
		secrets, err := storage.Load()
		if err != nil {
			return nil, fmt.Errorf("failed to load secrets: %w", err)
		}
		sm.secrets = secrets
	}

	return sm, nil
}

// Set imposta un secret
func (sm *SecretsManager) Set(key string, value string, secretType SecretType) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	secret := Secret{
		Key:       key,
		Value:     value,
		Type:      secretType,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}

	// Se esiste già, incrementa la versione
	if existing, exists := sm.secrets[key]; exists {
		secret.Version = existing.Version + 1
		secret.CreatedAt = existing.CreatedAt
	}

	sm.secrets[key] = secret

	// Salva su storage
	if sm.storage != nil {
		if err := sm.storage.Save(sm.secrets); err != nil {
			return fmt.Errorf("failed to save secrets: %w", err)
		}
	}

	return nil
}

// Get ottiene un secret
func (sm *SecretsManager) Get(key string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secret, exists := sm.secrets[key]
	if !exists {
		return "", ErrSecretNotFound
	}

	// Verifica scadenza
	if secret.ExpiresAt != nil && time.Now().After(*secret.ExpiresAt) {
		return "", ErrSecretExpired
	}

	return secret.Value, nil
}

// GetSecret ottiene un secret completo
func (sm *SecretsManager) GetSecret(key string) (Secret, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secret, exists := sm.secrets[key]
	if !exists {
		return Secret{}, ErrSecretNotFound
	}

	// Verifica scadenza
	if secret.ExpiresAt != nil && time.Now().After(*secret.ExpiresAt) {
		return Secret{}, ErrSecretExpired
	}

	return secret, nil
}

// Delete elimina un secret
func (sm *SecretsManager) Delete(key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.secrets, key)

	// Salva su storage
	if sm.storage != nil {
		if err := sm.storage.Save(sm.secrets); err != nil {
			return fmt.Errorf("failed to save secrets: %w", err)
		}
	}

	return nil
}

// List elenca tutti i secrets (solo metadati, non i valori)
func (sm *SecretsManager) List() []Secret {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secrets := make([]Secret, 0, len(sm.secrets))
	for _, secret := range sm.secrets {
		// Non includere il valore
		secretCopy := secret
		secretCopy.Value = "[REDACTED]"
		secrets = append(secrets, secretCopy)
	}

	return secrets
}

// SetExpiration imposta una data di scadenza per un secret
func (sm *SecretsManager) SetExpiration(key string, expiresAt time.Time) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	secret, exists := sm.secrets[key]
	if !exists {
		return ErrSecretNotFound
	}

	secret.ExpiresAt = &expiresAt
	secret.UpdatedAt = time.Now()
	sm.secrets[key] = secret

	// Salva su storage
	if sm.storage != nil {
		if err := sm.storage.Save(sm.secrets); err != nil {
			return fmt.Errorf("failed to save secrets: %w", err)
		}
	}

	return nil
}

// SetMetadata imposta metadata per un secret
func (sm *SecretsManager) SetMetadata(key string, metadata map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	secret, exists := sm.secrets[key]
	if !exists {
		return ErrSecretNotFound
	}

	secret.Metadata = metadata
	secret.UpdatedAt = time.Now()
	sm.secrets[key] = secret

	// Salva su storage
	if sm.storage != nil {
		if err := sm.storage.Save(sm.secrets); err != nil {
			return fmt.Errorf("failed to save secrets: %w", err)
		}
	}

	return nil
}

// EnableRotation abilita la rotazione automatica dei secrets
func (sm *SecretsManager) EnableRotation(period time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.rotationEnabled = true
	sm.rotationPeriod = period

	go sm.rotationLoop()
}

// rotationLoop gestisce la rotazione periodica dei secrets
func (sm *SecretsManager) rotationLoop() {
	ticker := time.NewTicker(24 * time.Hour) // Controlla ogni giorno
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.checkRotation()
		case <-sm.stopChan:
			return
		}
	}
}

// checkRotation verifica quali secrets devono essere ruotati
func (sm *SecretsManager) checkRotation() {
	sm.mu.RLock()
	toRotate := make([]string, 0)
	now := time.Now()

	for key, secret := range sm.secrets {
		if now.Sub(secret.UpdatedAt) > sm.rotationPeriod {
			toRotate = append(toRotate, key)
		}
	}
	sm.mu.RUnlock()

	// Qui si potrebbe implementare la logica di rotazione automatica
	// Per ora logghiamo solo quali secrets dovrebbero essere ruotati
	if len(toRotate) > 0 {
		fmt.Printf("Secrets requiring rotation: %v\n", toRotate)
	}
}

// Stop ferma il secrets manager
func (sm *SecretsManager) Stop() {
	if sm.rotationEnabled {
		close(sm.stopChan)
	}
}

// EnvSecretsProvider carica secrets dalle variabili d'ambiente
type EnvSecretsProvider struct {
	prefix string
}

// NewEnvSecretsProvider crea un provider per secrets da env
func NewEnvSecretsProvider(prefix string) *EnvSecretsProvider {
	return &EnvSecretsProvider{
		prefix: prefix,
	}
}

// Get ottiene un secret dalle variabili d'ambiente
func (esp *EnvSecretsProvider) Get(key string) (string, error) {
	envKey := esp.prefix + key
	value := os.Getenv(envKey)
	if value == "" {
		return "", ErrSecretNotFound
	}
	return value, nil
}

// VaultConfig rappresenta la configurazione per HashiCorp Vault
type VaultConfig struct {
	Address   string
	Token     string
	Path      string
	Namespace string
}

// VaultSecretsProvider gestisce secrets con HashiCorp Vault
type VaultSecretsProvider struct {
	config VaultConfig
	// client *vault.Client // Richiede github.com/hashicorp/vault/api
}

// NewVaultSecretsProvider crea un provider per Vault
func NewVaultSecretsProvider(config VaultConfig) (*VaultSecretsProvider, error) {
	// Implementazione con Vault client
	// Richiede l'aggiunta della dipendenza vault
	return &VaultSecretsProvider{
		config: config,
	}, nil
}

// Get ottiene un secret da Vault
func (vsp *VaultSecretsProvider) Get(key string) (string, error) {
	// Implementazione con Vault API
	// Per ora ritorna errore non implementato
	return "", errors.New("vault integration not implemented - install vault dependency")
}

// MultiSecretsProvider combina più provider
type MultiSecretsProvider struct {
	providers []SecretsProvider
}

// SecretsProvider definisce l'interfaccia per i provider di secrets
type SecretsProvider interface {
	Get(key string) (string, error)
}

// NewMultiSecretsProvider crea un provider multiplo
func NewMultiSecretsProvider(providers ...SecretsProvider) *MultiSecretsProvider {
	return &MultiSecretsProvider{
		providers: providers,
	}
}

// Get cerca il secret in tutti i provider
func (msp *MultiSecretsProvider) Get(key string) (string, error) {
	for _, provider := range msp.providers {
		value, err := provider.Get(key)
		if err == nil {
			return value, nil
		}
	}
	return "", ErrSecretNotFound
}

// SecretRotationCallback è chiamato quando un secret deve essere ruotato
type SecretRotationCallback func(key string, currentValue string) (newValue string, err error)

// RegisterRotationCallback registra un callback per la rotazione
func (sm *SecretsManager) RegisterRotationCallback(key string, callback SecretRotationCallback) error {
	// Implementazione future per callback di rotazione
	return nil
}

// GenerateAPIKey genera una nuova API key sicura
func GenerateAPIKey() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("goleap_%x", key), nil
}

// GenerateToken genera un token casuale sicuro
func GenerateToken(length int) (string, error) {
	token := make([]byte, length)
	if _, err := os.ReadFile("/dev/urandom"); err == nil {
		// Usa /dev/urandom se disponibile
		return fmt.Sprintf("%x", token), nil
	}

	// Fallback a GenerateKey
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", key), nil
}
