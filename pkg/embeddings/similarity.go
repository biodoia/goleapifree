package embeddings

import (
	"math"
)

// CosineSimilarity calcola la similarità coseno tra due vettori
// Restituisce un valore tra -1 e 1, dove 1 indica vettori identici
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, magnitudeA, magnitudeB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		magnitudeA += float64(a[i]) * float64(a[i])
		magnitudeB += float64(b[i]) * float64(b[i])
	}

	if magnitudeA == 0 || magnitudeB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(magnitudeA) * math.Sqrt(magnitudeB))
}

// DotProduct calcola il prodotto scalare tra due vettori
func DotProduct(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var result float64
	for i := range a {
		result += float64(a[i]) * float64(b[i])
	}
	return result
}

// EuclideanDistance calcola la distanza euclidea tra due vettori
// Minore è la distanza, più simili sono i vettori
func EuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a {
		diff := float64(a[i] - b[i])
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

// ManhattanDistance calcola la distanza Manhattan tra due vettori
func ManhattanDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a {
		sum += math.Abs(float64(a[i] - b[i]))
	}
	return sum
}

// Normalize normalizza un vettore a lunghezza unitaria
func Normalize(vec []float32) []float32 {
	var magnitude float64
	for _, v := range vec {
		magnitude += float64(v) * float64(v)
	}

	if magnitude == 0 {
		return vec
	}

	magnitude = math.Sqrt(magnitude)
	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = float32(float64(v) / magnitude)
	}
	return normalized
}

// SimilarityMetric rappresenta una funzione di metrica di similarità
type SimilarityMetric func(a, b []float32) float64

// MetricType rappresenta il tipo di metrica da utilizzare
type MetricType string

const (
	MetricCosine     MetricType = "cosine"
	MetricDotProduct MetricType = "dot_product"
	MetricEuclidean  MetricType = "euclidean"
	MetricManhattan  MetricType = "manhattan"
)

// GetMetric restituisce la funzione di metrica corrispondente al tipo
func GetMetric(metricType MetricType) SimilarityMetric {
	switch metricType {
	case MetricCosine:
		return CosineSimilarity
	case MetricDotProduct:
		return DotProduct
	case MetricEuclidean:
		return func(a, b []float32) float64 {
			// Invertiamo per avere valori più alti = più simili
			dist := EuclideanDistance(a, b)
			if dist == 0 {
				return 1.0
			}
			return 1.0 / (1.0 + dist)
		}
	case MetricManhattan:
		return func(a, b []float32) float64 {
			// Invertiamo per avere valori più alti = più simili
			dist := ManhattanDistance(a, b)
			if dist == 0 {
				return 1.0
			}
			return 1.0 / (1.0 + dist)
		}
	default:
		return CosineSimilarity
	}
}

// BatchCosineSimilarity calcola la similarità coseno tra un vettore e una lista di vettori
// Ottimizzato per performance
func BatchCosineSimilarity(query []float32, vectors [][]float32) []float64 {
	results := make([]float64, len(vectors))

	// Pre-calcola la magnitudine della query
	var queryMagnitude float64
	for _, v := range query {
		queryMagnitude += float64(v) * float64(v)
	}
	queryMagnitude = math.Sqrt(queryMagnitude)

	if queryMagnitude == 0 {
		return results
	}

	for i, vec := range vectors {
		if len(vec) != len(query) {
			results[i] = 0
			continue
		}

		var dotProduct, vecMagnitude float64
		for j := range query {
			dotProduct += float64(query[j]) * float64(vec[j])
			vecMagnitude += float64(vec[j]) * float64(vec[j])
		}

		if vecMagnitude == 0 {
			results[i] = 0
			continue
		}

		results[i] = dotProduct / (queryMagnitude * math.Sqrt(vecMagnitude))
	}

	return results
}

// TopKSimilar trova i K vettori più simili al vettore query
func TopKSimilar(query []float32, vectors [][]float32, k int, metric SimilarityMetric) []SimilarityResult {
	if k <= 0 || len(vectors) == 0 {
		return nil
	}

	if k > len(vectors) {
		k = len(vectors)
	}

	results := make([]SimilarityResult, len(vectors))
	for i, vec := range vectors {
		results[i] = SimilarityResult{
			Index:      i,
			Similarity: metric(query, vec),
		}
	}

	// Partial sort: trova i top K usando selection sort parziale
	for i := 0; i < k; i++ {
		maxIdx := i
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[maxIdx].Similarity {
				maxIdx = j
			}
		}
		if maxIdx != i {
			results[i], results[maxIdx] = results[maxIdx], results[i]
		}
	}

	return results[:k]
}

// SimilarityResult rappresenta il risultato di una ricerca di similarità
type SimilarityResult struct {
	Index      int
	Similarity float64
}
