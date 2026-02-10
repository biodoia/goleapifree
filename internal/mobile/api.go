package mobile

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIResponse rappresenta una risposta API mobile-optimized
type APIResponse struct {
	Data       interface{}       `json:"data,omitempty"`
	Meta       *ResponseMeta     `json:"meta,omitempty"`
	Pagination *PaginationInfo   `json:"pagination,omitempty"`
	Links      *ResponseLinks    `json:"links,omitempty"`
	Error      *ErrorResponse    `json:"error,omitempty"`
}

// ResponseMeta contiene metadata della risposta
type ResponseMeta struct {
	Timestamp     int64  `json:"timestamp"`
	RequestID     string `json:"request_id"`
	Version       string `json:"version"`
	ServerTime    int64  `json:"server_time"`
	ProcessingTime int64 `json:"processing_time_ms"`
}

// PaginationInfo contiene informazioni di paginazione
type PaginationInfo struct {
	CurrentPage  int   `json:"current_page"`
	PerPage      int   `json:"per_page"`
	TotalPages   int   `json:"total_pages"`
	TotalRecords int64 `json:"total_records"`
	HasNext      bool  `json:"has_next"`
	HasPrev      bool  `json:"has_prev"`
}

// ResponseLinks contiene link HATEOAS
type ResponseLinks struct {
	Self  string `json:"self,omitempty"`
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
	Next  string `json:"next,omitempty"`
	Prev  string `json:"prev,omitempty"`
}

// ErrorResponse rappresenta un errore API
type ErrorResponse struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PaginationParams contiene i parametri di paginazione
type PaginationParams struct {
	Page    int
	PerPage int
	Offset  int
}

// FieldSelector gestisce la selezione di campi sparsi
type FieldSelector struct {
	Fields map[string][]string
}

// MobileAPIHandler gestisce le richieste API mobile
type MobileAPIHandler struct {
	MaxPageSize     int
	DefaultPageSize int
	EnableGzip      bool
	APIVersion      string
}

// NewMobileAPIHandler crea un nuovo handler API mobile
func NewMobileAPIHandler() *MobileAPIHandler {
	return &MobileAPIHandler{
		MaxPageSize:     100,
		DefaultPageSize: 20,
		EnableGzip:      true,
		APIVersion:      "1.0",
	}
}

// ParsePaginationParams estrae i parametri di paginazione dalla request
func (h *MobileAPIHandler) ParsePaginationParams(r *http.Request) PaginationParams {
	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	perPage := h.DefaultPageSize
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 {
		perPage = pp
		if perPage > h.MaxPageSize {
			perPage = h.MaxPageSize
		}
	}

	offset := (page - 1) * perPage

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
		Offset:  offset,
	}
}

// ParseFieldSelector estrae i campi richiesti (sparse fieldsets)
func (h *MobileAPIHandler) ParseFieldSelector(r *http.Request) *FieldSelector {
	fieldsParam := r.URL.Query().Get("fields")
	if fieldsParam == "" {
		return nil
	}

	selector := &FieldSelector{
		Fields: make(map[string][]string),
	}

	// Supporta formato: fields[users]=id,name,email&fields[posts]=title,body
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "fields[") && strings.HasSuffix(key, "]") {
			resourceType := key[7 : len(key)-1]
			if len(values) > 0 {
				fields := strings.Split(values[0], ",")
				selector.Fields[resourceType] = fields
			}
		}
	}

	// Supporta anche formato semplice: fields=id,name,email
	if len(selector.Fields) == 0 && fieldsParam != "" {
		selector.Fields["default"] = strings.Split(fieldsParam, ",")
	}

	return selector
}

// FilterFields filtra i dati secondo i campi selezionati
func (fs *FieldSelector) FilterFields(data interface{}, resourceType string) interface{} {
	if fs == nil {
		return data
	}

	fields, ok := fs.Fields[resourceType]
	if !ok {
		fields, ok = fs.Fields["default"]
		if !ok {
			return data
		}
	}

	// Converti in map per filtrare
	jsonData, err := json.Marshal(data)
	if err != nil {
		return data
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return data
	}

	// Crea una nuova map solo con i campi richiesti
	filtered := make(map[string]interface{})
	for _, field := range fields {
		if value, exists := dataMap[field]; exists {
			filtered[field] = value
		}
	}

	return filtered
}

// BuildPaginationInfo costruisce le informazioni di paginazione
func (h *MobileAPIHandler) BuildPaginationInfo(params PaginationParams, totalRecords int64) *PaginationInfo {
	totalPages := int((totalRecords + int64(params.PerPage) - 1) / int64(params.PerPage))
	if totalPages == 0 {
		totalPages = 1
	}

	return &PaginationInfo{
		CurrentPage:  params.Page,
		PerPage:      params.PerPage,
		TotalPages:   totalPages,
		TotalRecords: totalRecords,
		HasNext:      params.Page < totalPages,
		HasPrev:      params.Page > 1,
	}
}

// BuildLinks costruisce i link HATEOAS per la paginazione
func (h *MobileAPIHandler) BuildLinks(r *http.Request, pagination *PaginationInfo) *ResponseLinks {
	if pagination == nil {
		return nil
	}

	baseURL := r.URL.Path
	query := r.URL.Query()

	links := &ResponseLinks{
		Self: buildURL(baseURL, query, pagination.CurrentPage),
	}

	if pagination.TotalPages > 0 {
		links.First = buildURL(baseURL, query, 1)
		links.Last = buildURL(baseURL, query, pagination.TotalPages)
	}

	if pagination.HasNext {
		links.Next = buildURL(baseURL, query, pagination.CurrentPage+1)
	}

	if pagination.HasPrev {
		links.Prev = buildURL(baseURL, query, pagination.CurrentPage-1)
	}

	return links
}

// buildURL costruisce un URL con parametri di query
func buildURL(baseURL string, query map[string][]string, page int) string {
	newQuery := make(map[string][]string)
	for k, v := range query {
		if k != "page" {
			newQuery[k] = v
		}
	}
	newQuery["page"] = []string{strconv.Itoa(page)}

	var params []string
	for k, v := range newQuery {
		if len(v) > 0 {
			params = append(params, k+"="+v[0])
		}
	}

	if len(params) > 0 {
		return baseURL + "?" + strings.Join(params, "&")
	}
	return baseURL
}

// RespondJSON invia una risposta JSON ottimizzata per mobile
func (h *MobileAPIHandler) RespondJSON(w http.ResponseWriter, r *http.Request, statusCode int, response *APIResponse, startTime time.Time) {
	// Aggiungi metadata
	if response.Meta == nil {
		response.Meta = &ResponseMeta{}
	}
	response.Meta.Timestamp = time.Now().Unix()
	response.Meta.Version = h.APIVersion
	response.Meta.ServerTime = time.Now().UnixMilli()
	response.Meta.ProcessingTime = time.Since(startTime).Milliseconds()

	// Request ID dal context o genera uno nuovo
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}
	response.Meta.RequestID = requestID

	// Headers ottimizzati per mobile
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("X-API-Version", h.APIVersion)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Supporto per compressione gzip
	var writer io.Writer = w
	if h.EnableGzip && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		writer = gz
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(writer).Encode(response)
}

// RespondSuccess risponde con successo
func (h *MobileAPIHandler) RespondSuccess(w http.ResponseWriter, r *http.Request, data interface{}, startTime time.Time) {
	response := &APIResponse{
		Data: data,
	}
	h.RespondJSON(w, r, http.StatusOK, response, startTime)
}

// RespondWithPagination risponde con dati paginati
func (h *MobileAPIHandler) RespondWithPagination(w http.ResponseWriter, r *http.Request, data interface{}, pagination *PaginationInfo, startTime time.Time) {
	response := &APIResponse{
		Data:       data,
		Pagination: pagination,
		Links:      h.BuildLinks(r, pagination),
	}
	h.RespondJSON(w, r, http.StatusOK, response, startTime)
}

// RespondError risponde con un errore
func (h *MobileAPIHandler) RespondError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details map[string]interface{}, startTime time.Time) {
	response := &APIResponse{
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	h.RespondJSON(w, r, statusCode, response, startTime)
}

// generateRequestID genera un ID univoco per la richiesta
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// CompressMiddleware middleware per la compressione automatica
func CompressMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()

		gzw := &gzipResponseWriter{Writer: gz, ResponseWriter: w}
		next.ServeHTTP(gzw, r)
	})
}

// gzipResponseWriter wrapper per ResponseWriter con gzip
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// OptimizedQueryParams contiene parametri di query ottimizzati
type OptimizedQueryParams struct {
	Pagination PaginationParams
	Fields     *FieldSelector
	Sort       []SortField
	Filters    map[string]interface{}
	Include    []string // Per caricamento eager di relazioni
}

// SortField rappresenta un campo di ordinamento
type SortField struct {
	Field string
	Desc  bool
}

// ParseOptimizedParams estrae tutti i parametri ottimizzati
func (h *MobileAPIHandler) ParseOptimizedParams(r *http.Request) *OptimizedQueryParams {
	params := &OptimizedQueryParams{
		Pagination: h.ParsePaginationParams(r),
		Fields:     h.ParseFieldSelector(r),
		Sort:       parseSortFields(r),
		Filters:    parseFilters(r),
		Include:    parseInclude(r),
	}
	return params
}

// parseSortFields estrae i campi di ordinamento
func parseSortFields(r *http.Request) []SortField {
	sortParam := r.URL.Query().Get("sort")
	if sortParam == "" {
		return nil
	}

	fields := strings.Split(sortParam, ",")
	sortFields := make([]SortField, 0, len(fields))

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		desc := strings.HasPrefix(field, "-")
		if desc {
			field = field[1:]
		}

		sortFields = append(sortFields, SortField{
			Field: field,
			Desc:  desc,
		})
	}

	return sortFields
}

// parseFilters estrae i filtri dalla query
func parseFilters(r *http.Request) map[string]interface{} {
	filters := make(map[string]interface{})

	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "filter[") && strings.HasSuffix(key, "]") {
			filterName := key[7 : len(key)-1]
			if len(values) > 0 {
				filters[filterName] = values[0]
			}
		}
	}

	return filters
}

// parseInclude estrae le relazioni da includere
func parseInclude(r *http.Request) []string {
	includeParam := r.URL.Query().Get("include")
	if includeParam == "" {
		return nil
	}

	includes := strings.Split(includeParam, ",")
	for i, inc := range includes {
		includes[i] = strings.TrimSpace(inc)
	}

	return includes
}

// BatchRequest rappresenta una richiesta batch
type BatchRequest struct {
	Requests []BatchRequestItem `json:"requests"`
}

// BatchRequestItem rappresenta un item della batch request
type BatchRequestItem struct {
	ID     string            `json:"id"`
	Method string            `json:"method"`
	URL    string            `json:"url"`
	Body   interface{}       `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// BatchResponse rappresenta una risposta batch
type BatchResponse struct {
	Responses []BatchResponseItem `json:"responses"`
}

// BatchResponseItem rappresenta un item della batch response
type BatchResponseItem struct {
	ID         string      `json:"id"`
	StatusCode int         `json:"status_code"`
	Body       interface{} `json:"body"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// HandleBatchRequest gestisce richieste batch per ridurre il numero di round-trip
func (h *MobileAPIHandler) HandleBatchRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	var batchReq BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
		h.RespondError(w, r, http.StatusBadRequest, "INVALID_BATCH", "Invalid batch request", nil, startTime)
		return
	}

	// Limita il numero di richieste batch
	if len(batchReq.Requests) > 10 {
		h.RespondError(w, r, http.StatusBadRequest, "BATCH_LIMIT_EXCEEDED", "Maximum 10 requests per batch", nil, startTime)
		return
	}

	batchResp := &BatchResponse{
		Responses: make([]BatchResponseItem, len(batchReq.Requests)),
	}

	// Esegui ogni richiesta (in questo esempio, simulato)
	for i, req := range batchReq.Requests {
		batchResp.Responses[i] = BatchResponseItem{
			ID:         req.ID,
			StatusCode: http.StatusOK,
			Body: map[string]interface{}{
				"message": "Batch request processed",
				"method":  req.Method,
				"url":     req.URL,
			},
		}
	}

	response := &APIResponse{
		Data: batchResp,
	}
	h.RespondJSON(w, r, http.StatusOK, response, startTime)
}
