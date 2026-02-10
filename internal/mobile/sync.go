package mobile

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SyncStatus rappresenta lo stato di sincronizzazione
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusSynced    SyncStatus = "synced"
	SyncStatusConflict  SyncStatus = "conflict"
	SyncStatusDeleted   SyncStatus = "deleted"
)

// ConflictResolution rappresenta la strategia di risoluzione conflitti
type ConflictResolution string

const (
	ResolutionServerWins ConflictResolution = "server_wins"
	ResolutionClientWins ConflictResolution = "client_wins"
	ResolutionManual     ConflictResolution = "manual"
	ResolutionMerge      ConflictResolution = "merge"
	ResolutionNewest     ConflictResolution = "newest"
)

// SyncEntity rappresenta un'entità sincronizzabile
type SyncEntity struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Data          map[string]interface{} `json:"data"`
	Version       int64                  `json:"version"`
	LastModified  time.Time              `json:"last_modified"`
	CreatedAt     time.Time              `json:"created_at"`
	DeletedAt     *time.Time             `json:"deleted_at,omitempty"`
	Checksum      string                 `json:"checksum"`
	DeviceID      string                 `json:"device_id"`
	UserID        string                 `json:"user_id"`
	SyncStatus    SyncStatus             `json:"sync_status"`
}

// SyncRequest rappresenta una richiesta di sincronizzazione
type SyncRequest struct {
	DeviceID      string              `json:"device_id"`
	UserID        string              `json:"user_id"`
	LastSyncTime  time.Time           `json:"last_sync_time"`
	Changes       []*SyncEntity       `json:"changes"`
	DeletedIDs    []string            `json:"deleted_ids"`
	EntityTypes   []string            `json:"entity_types"` // Filtra per tipo
}

// SyncResponse rappresenta una risposta di sincronizzazione
type SyncResponse struct {
	ServerTime    time.Time       `json:"server_time"`
	Changes       []*SyncEntity   `json:"changes"`
	Conflicts     []*SyncConflict `json:"conflicts,omitempty"`
	DeletedIDs    []string        `json:"deleted_ids"`
	NextSyncToken string          `json:"next_sync_token,omitempty"`
	HasMore       bool            `json:"has_more"`
}

// SyncConflict rappresenta un conflitto di sincronizzazione
type SyncConflict struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	ServerEntity *SyncEntity   `json:"server_entity"`
	ClientEntity *SyncEntity   `json:"client_entity"`
	Resolution   ConflictResolution `json:"resolution,omitempty"`
	ResolvedData map[string]interface{} `json:"resolved_data,omitempty"`
}

// DeltaSyncRequest rappresenta una richiesta di delta sync
type DeltaSyncRequest struct {
	DeviceID     string    `json:"device_id"`
	UserID       string    `json:"user_id"`
	SyncToken    string    `json:"sync_token,omitempty"`
	EntityTypes  []string  `json:"entity_types"`
	Limit        int       `json:"limit"`
}

// DeltaSyncResponse rappresenta una risposta di delta sync
type DeltaSyncResponse struct {
	ServerTime    time.Time     `json:"server_time"`
	Added         []*SyncEntity `json:"added"`
	Modified      []*SyncEntity `json:"modified"`
	Deleted       []string      `json:"deleted"`
	NextSyncToken string        `json:"next_sync_token"`
	HasMore       bool          `json:"has_more"`
}

// SyncService gestisce la sincronizzazione offline
type SyncService struct {
	entities         map[string]*SyncEntity // key: entity_id
	userEntities     map[string][]string    // key: user_id, value: entity_ids
	conflicts        map[string]*SyncConflict
	syncTokens       map[string]time.Time   // key: sync_token, value: timestamp
	mu               sync.RWMutex
	conflictResolver ConflictResolver
	deltaStore       *DeltaStore
}

// ConflictResolver è l'interfaccia per risolvere i conflitti
type ConflictResolver interface {
	Resolve(conflict *SyncConflict) (*SyncEntity, error)
}

// DefaultConflictResolver implementazione di default
type DefaultConflictResolver struct {
	Strategy ConflictResolution
}

// Resolve risolve un conflitto secondo la strategia
func (r *DefaultConflictResolver) Resolve(conflict *SyncConflict) (*SyncEntity, error) {
	switch r.Strategy {
	case ResolutionServerWins:
		return conflict.ServerEntity, nil
	case ResolutionClientWins:
		return conflict.ClientEntity, nil
	case ResolutionNewest:
		if conflict.ClientEntity.LastModified.After(conflict.ServerEntity.LastModified) {
			return conflict.ClientEntity, nil
		}
		return conflict.ServerEntity, nil
	case ResolutionMerge:
		return r.mergeEntities(conflict.ServerEntity, conflict.ClientEntity), nil
	default:
		return nil, errors.New("manual resolution required")
	}
}

// mergeEntities unisce due entità
func (r *DefaultConflictResolver) mergeEntities(server, client *SyncEntity) *SyncEntity {
	merged := &SyncEntity{
		ID:           server.ID,
		Type:         server.Type,
		Data:         make(map[string]interface{}),
		Version:      server.Version + 1,
		LastModified: time.Now(),
		CreatedAt:    server.CreatedAt,
		UserID:       server.UserID,
		DeviceID:     server.DeviceID,
		SyncStatus:   SyncStatusSynced,
	}

	// Copia dati dal server
	for k, v := range server.Data {
		merged.Data[k] = v
	}

	// Sovrascrivi con dati client se più recenti
	for k, v := range client.Data {
		merged.Data[k] = v
	}

	merged.Checksum = calculateChecksum(merged.Data)
	return merged
}

// DeltaStore memorizza i delta per la sincronizzazione incrementale
type DeltaStore struct {
	deltas map[string][]*SyncDelta // key: user_id
	mu     sync.RWMutex
}

// SyncDelta rappresenta un cambiamento
type SyncDelta struct {
	EntityID  string    `json:"entity_id"`
	Type      string    `json:"type"`
	Operation string    `json:"operation"` // "create", "update", "delete"
	Timestamp time.Time `json:"timestamp"`
	Version   int64     `json:"version"`
}

// NewSyncService crea un nuovo servizio di sincronizzazione
func NewSyncService() *SyncService {
	return &SyncService{
		entities:     make(map[string]*SyncEntity),
		userEntities: make(map[string][]string),
		conflicts:    make(map[string]*SyncConflict),
		syncTokens:   make(map[string]time.Time),
		conflictResolver: &DefaultConflictResolver{
			Strategy: ResolutionNewest,
		},
		deltaStore: &DeltaStore{
			deltas: make(map[string][]*SyncDelta),
		},
	}
}

// Sync esegue una sincronizzazione completa
func (ss *SyncService) Sync(ctx context.Context, req *SyncRequest) (*SyncResponse, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	resp := &SyncResponse{
		ServerTime: time.Now(),
		Changes:    make([]*SyncEntity, 0),
		Conflicts:  make([]*SyncConflict, 0),
		DeletedIDs: make([]string, 0),
	}

	// Processa le modifiche del client
	for _, clientEntity := range req.Changes {
		if err := ss.processClientChange(clientEntity, resp); err != nil {
			return nil, err
		}
	}

	// Processa le eliminazioni del client
	for _, deletedID := range req.DeletedIDs {
		ss.processClientDeletion(deletedID, req.UserID)
	}

	// Ottieni le modifiche dal server per il client
	serverChanges := ss.getServerChanges(req.UserID, req.LastSyncTime, req.EntityTypes)
	resp.Changes = serverChanges

	// Ottieni le eliminazioni dal server
	resp.DeletedIDs = ss.getServerDeletions(req.UserID, req.LastSyncTime)

	// Genera token per la prossima sincronizzazione
	resp.NextSyncToken = ss.generateSyncToken(resp.ServerTime)

	return resp, nil
}

// DeltaSync esegue una sincronizzazione incrementale (delta sync)
func (ss *SyncService) DeltaSync(ctx context.Context, req *DeltaSyncRequest) (*DeltaSyncResponse, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var sinceTime time.Time
	if req.SyncToken != "" {
		if t, exists := ss.syncTokens[req.SyncToken]; exists {
			sinceTime = t
		}
	}

	resp := &DeltaSyncResponse{
		ServerTime: time.Now(),
		Added:      make([]*SyncEntity, 0),
		Modified:   make([]*SyncEntity, 0),
		Deleted:    make([]string, 0),
	}

	// Ottieni i delta per l'utente
	deltas := ss.deltaStore.GetDeltasSince(req.UserID, sinceTime)

	limit := req.Limit
	if limit == 0 {
		limit = 100
	}

	count := 0
	for _, delta := range deltas {
		if count >= limit {
			resp.HasMore = true
			break
		}

		switch delta.Operation {
		case "create":
			if entity, exists := ss.entities[delta.EntityID]; exists {
				resp.Added = append(resp.Added, entity)
				count++
			}
		case "update":
			if entity, exists := ss.entities[delta.EntityID]; exists {
				resp.Modified = append(resp.Modified, entity)
				count++
			}
		case "delete":
			resp.Deleted = append(resp.Deleted, delta.EntityID)
			count++
		}
	}

	resp.NextSyncToken = ss.generateSyncToken(resp.ServerTime)
	return resp, nil
}

// processClientChange processa un cambiamento dal client
func (ss *SyncService) processClientChange(clientEntity *SyncEntity, resp *SyncResponse) error {
	serverEntity, exists := ss.entities[clientEntity.ID]

	if !exists {
		// Nuova entità dal client
		clientEntity.Version = 1
		clientEntity.LastModified = time.Now()
		clientEntity.SyncStatus = SyncStatusSynced
		clientEntity.Checksum = calculateChecksum(clientEntity.Data)
		ss.entities[clientEntity.ID] = clientEntity
		ss.addToUserEntities(clientEntity.UserID, clientEntity.ID)
		ss.deltaStore.AddDelta(clientEntity.UserID, &SyncDelta{
			EntityID:  clientEntity.ID,
			Type:      clientEntity.Type,
			Operation: "create",
			Timestamp: time.Now(),
			Version:   clientEntity.Version,
		})
		return nil
	}

	// Entità esistente - verifica conflitti
	if ss.hasConflict(serverEntity, clientEntity) {
		conflict := &SyncConflict{
			ID:           clientEntity.ID,
			Type:         clientEntity.Type,
			ServerEntity: serverEntity,
			ClientEntity: clientEntity,
		}

		// Risolvi il conflitto
		resolved, err := ss.conflictResolver.Resolve(conflict)
		if err != nil {
			// Conflitto richiede risoluzione manuale
			resp.Conflicts = append(resp.Conflicts, conflict)
			ss.conflicts[clientEntity.ID] = conflict
			return nil
		}

		// Applica la risoluzione
		ss.entities[clientEntity.ID] = resolved
		ss.deltaStore.AddDelta(clientEntity.UserID, &SyncDelta{
			EntityID:  clientEntity.ID,
			Type:      clientEntity.Type,
			Operation: "update",
			Timestamp: time.Now(),
			Version:   resolved.Version,
		})
	} else {
		// Nessun conflitto - aggiorna
		clientEntity.Version = serverEntity.Version + 1
		clientEntity.LastModified = time.Now()
		clientEntity.SyncStatus = SyncStatusSynced
		clientEntity.Checksum = calculateChecksum(clientEntity.Data)
		ss.entities[clientEntity.ID] = clientEntity
		ss.deltaStore.AddDelta(clientEntity.UserID, &SyncDelta{
			EntityID:  clientEntity.ID,
			Type:      clientEntity.Type,
			Operation: "update",
			Timestamp: time.Now(),
			Version:   clientEntity.Version,
		})
	}

	return nil
}

// hasConflict verifica se c'è un conflitto tra server e client
func (ss *SyncService) hasConflict(server, client *SyncEntity) bool {
	// Verifica se il client ha una versione obsoleta
	if client.Version < server.Version {
		// Verifica se i checksum sono diversi
		if server.Checksum != client.Checksum {
			return true
		}
	}
	return false
}

// processClientDeletion processa un'eliminazione dal client
func (ss *SyncService) processClientDeletion(entityID, userID string) {
	if entity, exists := ss.entities[entityID]; exists {
		now := time.Now()
		entity.DeletedAt = &now
		entity.SyncStatus = SyncStatusDeleted
		ss.deltaStore.AddDelta(userID, &SyncDelta{
			EntityID:  entityID,
			Type:      entity.Type,
			Operation: "delete",
			Timestamp: now,
			Version:   entity.Version,
		})
	}
}

// getServerChanges ottiene i cambiamenti dal server
func (ss *SyncService) getServerChanges(userID string, since time.Time, entityTypes []string) []*SyncEntity {
	changes := make([]*SyncEntity, 0)

	entityIDs, exists := ss.userEntities[userID]
	if !exists {
		return changes
	}

	typeFilter := make(map[string]bool)
	for _, t := range entityTypes {
		typeFilter[t] = true
	}

	for _, entityID := range entityIDs {
		entity, exists := ss.entities[entityID]
		if !exists || entity.DeletedAt != nil {
			continue
		}

		// Filtra per tipo se specificato
		if len(typeFilter) > 0 && !typeFilter[entity.Type] {
			continue
		}

		// Aggiungi solo se modificato dopo l'ultima sincronizzazione
		if entity.LastModified.After(since) {
			changes = append(changes, entity)
		}
	}

	return changes
}

// getServerDeletions ottiene le eliminazioni dal server
func (ss *SyncService) getServerDeletions(userID string, since time.Time) []string {
	deletions := make([]string, 0)

	entityIDs, exists := ss.userEntities[userID]
	if !exists {
		return deletions
	}

	for _, entityID := range entityIDs {
		entity, exists := ss.entities[entityID]
		if !exists {
			continue
		}

		if entity.DeletedAt != nil && entity.DeletedAt.After(since) {
			deletions = append(deletions, entityID)
		}
	}

	return deletions
}

// addToUserEntities aggiunge un'entità all'indice utente
func (ss *SyncService) addToUserEntities(userID, entityID string) {
	if _, exists := ss.userEntities[userID]; !exists {
		ss.userEntities[userID] = make([]string, 0)
	}
	ss.userEntities[userID] = append(ss.userEntities[userID], entityID)
}

// generateSyncToken genera un token di sincronizzazione
func (ss *SyncService) generateSyncToken(timestamp time.Time) string {
	token := fmt.Sprintf("sync_%d", timestamp.UnixNano())
	ss.syncTokens[token] = timestamp
	return token
}

// calculateChecksum calcola il checksum dei dati
func calculateChecksum(data map[string]interface{}) string {
	jsonData, _ := json.Marshal(data)
	hash := md5.Sum(jsonData)
	return hex.EncodeToString(hash[:])
}

// AddDelta aggiunge un delta allo store
func (ds *DeltaStore) AddDelta(userID string, delta *SyncDelta) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if _, exists := ds.deltas[userID]; !exists {
		ds.deltas[userID] = make([]*SyncDelta, 0)
	}
	ds.deltas[userID] = append(ds.deltas[userID], delta)
}

// GetDeltasSince ottiene i delta da un certo timestamp
func (ds *DeltaStore) GetDeltasSince(userID string, since time.Time) []*SyncDelta {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	userDeltas, exists := ds.deltas[userID]
	if !exists {
		return []*SyncDelta{}
	}

	result := make([]*SyncDelta, 0)
	for _, delta := range userDeltas {
		if delta.Timestamp.After(since) {
			result = append(result, delta)
		}
	}

	return result
}

// ResolveConflict risolve manualmente un conflitto
func (ss *SyncService) ResolveConflict(ctx context.Context, conflictID string, resolution ConflictResolution, resolvedData map[string]interface{}) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	conflict, exists := ss.conflicts[conflictID]
	if !exists {
		return errors.New("conflict not found")
	}

	var resolved *SyncEntity

	switch resolution {
	case ResolutionServerWins:
		resolved = conflict.ServerEntity
	case ResolutionClientWins:
		resolved = conflict.ClientEntity
	case ResolutionManual:
		resolved = &SyncEntity{
			ID:           conflict.ID,
			Type:         conflict.Type,
			Data:         resolvedData,
			Version:      conflict.ServerEntity.Version + 1,
			LastModified: time.Now(),
			CreatedAt:    conflict.ServerEntity.CreatedAt,
			UserID:       conflict.ServerEntity.UserID,
			DeviceID:     conflict.ServerEntity.DeviceID,
			SyncStatus:   SyncStatusSynced,
		}
		resolved.Checksum = calculateChecksum(resolved.Data)
	default:
		return errors.New("invalid resolution strategy")
	}

	ss.entities[conflictID] = resolved
	delete(ss.conflicts, conflictID)

	return nil
}

// GetEntity ottiene un'entità per ID
func (ss *SyncService) GetEntity(entityID string) (*SyncEntity, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	entity, exists := ss.entities[entityID]
	if !exists {
		return nil, errors.New("entity not found")
	}

	return entity, nil
}

// CreateEntity crea una nuova entità
func (ss *SyncService) CreateEntity(ctx context.Context, entity *SyncEntity) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	entity.Version = 1
	entity.CreatedAt = time.Now()
	entity.LastModified = time.Now()
	entity.SyncStatus = SyncStatusSynced
	entity.Checksum = calculateChecksum(entity.Data)

	ss.entities[entity.ID] = entity
	ss.addToUserEntities(entity.UserID, entity.ID)

	ss.deltaStore.AddDelta(entity.UserID, &SyncDelta{
		EntityID:  entity.ID,
		Type:      entity.Type,
		Operation: "create",
		Timestamp: time.Now(),
		Version:   entity.Version,
	})

	return nil
}

// UpdateEntity aggiorna un'entità esistente
func (ss *SyncService) UpdateEntity(ctx context.Context, entityID string, data map[string]interface{}) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	entity, exists := ss.entities[entityID]
	if !exists {
		return errors.New("entity not found")
	}

	entity.Data = data
	entity.Version++
	entity.LastModified = time.Now()
	entity.Checksum = calculateChecksum(data)

	ss.deltaStore.AddDelta(entity.UserID, &SyncDelta{
		EntityID:  entityID,
		Type:      entity.Type,
		Operation: "update",
		Timestamp: time.Now(),
		Version:   entity.Version,
	})

	return nil
}

// DeleteEntity elimina un'entità
func (ss *SyncService) DeleteEntity(ctx context.Context, entityID string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	entity, exists := ss.entities[entityID]
	if !exists {
		return errors.New("entity not found")
	}

	now := time.Now()
	entity.DeletedAt = &now
	entity.SyncStatus = SyncStatusDeleted

	ss.deltaStore.AddDelta(entity.UserID, &SyncDelta{
		EntityID:  entityID,
		Type:      entity.Type,
		Operation: "delete",
		Timestamp: now,
		Version:   entity.Version,
	})

	return nil
}
