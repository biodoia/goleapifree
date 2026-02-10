package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

var (
	// ErrInvalidKeySize viene restituito quando la chiave non ha la lunghezza corretta
	ErrInvalidKeySize = errors.New("invalid key size: must be 32 bytes for AES-256")

	// ErrInvalidCiphertext viene restituito quando il ciphertext è troppo corto
	ErrInvalidCiphertext = errors.New("ciphertext too short")

	// ErrDecryptionFailed viene restituito quando la decryption fallisce
	ErrDecryptionFailed = errors.New("decryption failed")
)

const (
	// AES256KeySize è la dimensione della chiave per AES-256
	AES256KeySize = 32

	// PBKDF2Iterations è il numero di iterazioni per PBKDF2
	PBKDF2Iterations = 100000

	// SaltSize è la dimensione del salt per PBKDF2
	SaltSize = 32
)

// EncryptionManager gestisce le operazioni di encryption/decryption
type EncryptionManager struct {
	masterKey []byte
}

// NewEncryptionManager crea un nuovo manager per encryption
func NewEncryptionManager(masterKey string) (*EncryptionManager, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("master key cannot be empty")
	}

	// Deriva la chiave dal master key usando SHA-256
	hash := sha256.Sum256([]byte(masterKey))

	return &EncryptionManager{
		masterKey: hash[:],
	}, nil
}

// DeriveKey deriva una chiave da una password usando PBKDF2
func DeriveKey(password string, salt []byte) []byte {
	if len(salt) == 0 {
		salt = make([]byte, SaltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			panic(fmt.Sprintf("failed to generate salt: %v", err))
		}
	}

	return pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, AES256KeySize, sha256.New)
}

// GenerateSalt genera un salt casuale per PBKDF2
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateKey genera una chiave casuale per AES-256
func GenerateKey() ([]byte, error) {
	key := make([]byte, AES256KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// Encrypt cifra i dati usando AES-256-GCM
func (em *EncryptionManager) Encrypt(plaintext []byte) (string, error) {
	return Encrypt(plaintext, em.masterKey)
}

// Decrypt decifra i dati usando AES-256-GCM
func (em *EncryptionManager) Decrypt(ciphertext string) ([]byte, error) {
	return Decrypt(ciphertext, em.masterKey)
}

// Encrypt cifra i dati usando AES-256-GCM con la chiave fornita
func Encrypt(plaintext, key []byte) (string, error) {
	if len(key) != AES256KeySize {
		return "", ErrInvalidKeySize
	}

	// Crea il cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Crea GCM (Galois/Counter Mode)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Genera un nonce casuale
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Cifra i dati (nonce || ciphertext || tag)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Codifica in base64 per storage/trasporto
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decifra i dati usando AES-256-GCM con la chiave fornita
func Decrypt(ciphertextBase64 string, key []byte) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, ErrInvalidKeySize
	}

	// Decodifica da base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Crea il cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Crea GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Verifica la lunghezza minima
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Estrai nonce e ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decifra i dati
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString cifra una stringa e ritorna base64
func (em *EncryptionManager) EncryptString(plaintext string) (string, error) {
	return em.Encrypt([]byte(plaintext))
}

// DecryptString decifra una stringa base64
func (em *EncryptionManager) DecryptString(ciphertext string) (string, error) {
	plaintext, err := em.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// HashPassword crea un hash sicuro della password usando PBKDF2
func HashPassword(password string, salt []byte) string {
	if len(salt) == 0 {
		salt = make([]byte, SaltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			panic(fmt.Sprintf("failed to generate salt: %v", err))
		}
	}

	hash := pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, 64, sha256.New)

	// Combina salt e hash per storage
	combined := append(salt, hash...)
	return hex.EncodeToString(combined)
}

// VerifyPassword verifica una password contro il suo hash
func VerifyPassword(password, hashedPassword string) bool {
	// Decodifica l'hash combinato
	combined, err := hex.DecodeString(hashedPassword)
	if err != nil || len(combined) < SaltSize {
		return false
	}

	// Estrai salt e hash
	salt := combined[:SaltSize]
	expectedHash := combined[SaltSize:]

	// Calcola l'hash della password fornita
	hash := pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, 64, sha256.New)

	// Confronto constant-time per prevenire timing attacks
	return constantTimeCompare(hash, expectedHash)
}

// constantTimeCompare confronta due slice in tempo costante
func constantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}

	return result == 0
}

// SecureKey rappresenta una chiave sicura in memoria
type SecureKey struct {
	key []byte
}

// NewSecureKey crea una nuova chiave sicura
func NewSecureKey(key []byte) *SecureKey {
	// Copia la chiave per evitare modifiche esterne
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	return &SecureKey{
		key: keyCopy,
	}
}

// Key ritorna la chiave (solo per uso interno)
func (sk *SecureKey) Key() []byte {
	return sk.key
}

// Destroy cancella la chiave dalla memoria
func (sk *SecureKey) Destroy() {
	// Sovrascrivi la chiave con zeri
	for i := range sk.key {
		sk.key[i] = 0
	}
	sk.key = nil
}

// EncryptCredentials cifra credenziali sensibili
func (em *EncryptionManager) EncryptCredentials(username, password string) (string, error) {
	credentials := fmt.Sprintf("%s:%s", username, password)
	return em.EncryptString(credentials)
}

// DecryptCredentials decifra credenziali sensibili
func (em *EncryptionManager) DecryptCredentials(encrypted string) (username, password string, err error) {
	credentials, err := em.DecryptString(encrypted)
	if err != nil {
		return "", "", err
	}

	// Split username:password
	for i := 0; i < len(credentials); i++ {
		if credentials[i] == ':' {
			return credentials[:i], credentials[i+1:], nil
		}
	}

	return "", "", errors.New("invalid credentials format")
}
