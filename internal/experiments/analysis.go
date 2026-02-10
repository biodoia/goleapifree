package experiments

import (
	"math"
	"sort"

	"github.com/rs/zerolog/log"
)

// StatisticalAnalyzer esegue analisi statistiche sugli esperimenti
type StatisticalAnalyzer struct {
	confidenceLevel float64 // Default: 0.95 (95%)
}

// NewStatisticalAnalyzer crea un nuovo analyzer
func NewStatisticalAnalyzer(confidenceLevel float64) *StatisticalAnalyzer {
	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		confidenceLevel = 0.95
	}

	return &StatisticalAnalyzer{
		confidenceLevel: confidenceLevel,
	}
}

// AnalyzeExperiment esegue l'analisi statistica completa di un esperimento
func (sa *StatisticalAnalyzer) AnalyzeExperiment(exp *Experiment) *StatisticalAnalysis {
	if exp.Results.VariantStats == nil || len(exp.Results.VariantStats) < 2 {
		return nil
	}

	analysis := &StatisticalAnalysis{
		ConfidenceLevel: sa.confidenceLevel,
		Comparisons:     make([]VariantComparison, 0),
	}

	// Identifica variante di controllo
	var controlVariant *VariantStatistics
	var controlID string
	for _, variant := range exp.Variants {
		if variant.IsControl {
			controlID = variant.ID
			controlVariant = exp.Results.VariantStats[variant.ID]
			break
		}
	}

	// Se non c'è controllo, usa la prima variante
	if controlVariant == nil && len(exp.Variants) > 0 {
		controlID = exp.Variants[0].ID
		controlVariant = exp.Results.VariantStats[controlID]
	}

	if controlVariant == nil {
		return nil
	}

	// Confronta ogni variante con il controllo
	bestVariantID := controlID
	bestMetricValue := sa.getMetricValue(controlVariant, exp.TargetMetric)
	maxSignificantDiff := 0.0

	for _, variant := range exp.Variants {
		if variant.ID == controlID {
			continue
		}

		variantStats := exp.Results.VariantStats[variant.ID]
		if variantStats == nil {
			continue
		}

		// Esegui test statistico appropriato
		var comparison VariantComparison
		switch exp.TargetMetric {
		case "success_rate":
			comparison = sa.ChiSquareTest(controlVariant, variantStats, controlID, variant.ID)
		case "latency", "cost":
			comparison = sa.TTest(controlVariant, variantStats, controlID, variant.ID, exp.TargetMetric)
		default:
			comparison = sa.TTest(controlVariant, variantStats, controlID, variant.ID, "success_rate")
		}

		analysis.Comparisons = append(analysis.Comparisons, comparison)

		// Determina se questo è il vincitore
		if comparison.IsSignificant {
			variantValue := sa.getMetricValue(variantStats, exp.TargetMetric)

			// Per latency e cost, più basso è meglio
			isLowerBetter := exp.TargetMetric == "latency" || exp.TargetMetric == "cost"

			if isLowerBetter {
				if variantValue < bestMetricValue {
					improvement := ((bestMetricValue - variantValue) / bestMetricValue) * 100
					if improvement > maxSignificantDiff {
						bestVariantID = variant.ID
						bestMetricValue = variantValue
						maxSignificantDiff = improvement
					}
				}
			} else {
				if variantValue > bestMetricValue {
					improvement := ((variantValue - bestMetricValue) / bestMetricValue) * 100
					if improvement > maxSignificantDiff {
						bestVariantID = variant.ID
						bestMetricValue = variantValue
						maxSignificantDiff = improvement
					}
				}
			}
		}
	}

	// Determina significatività generale
	analysis.IsSignificant = false
	for _, comp := range analysis.Comparisons {
		if comp.IsSignificant {
			analysis.IsSignificant = true
			break
		}
	}

	// Calcola p-value globale (minimo dei p-value)
	analysis.PValue = 1.0
	for _, comp := range analysis.Comparisons {
		if comp.PValue < analysis.PValue {
			analysis.PValue = comp.PValue
		}
	}

	// Determina test type usato
	if exp.TargetMetric == "success_rate" {
		analysis.TestType = "chi_square"
	} else {
		analysis.TestType = "t_test"
	}

	// Calcola effect size (Cohen's d per il vincitore)
	if bestVariantID != controlID {
		bestStats := exp.Results.VariantStats[bestVariantID]
		analysis.EffectSize = sa.CalculateCohenD(controlVariant, bestStats, exp.TargetMetric)
	}

	// Raccomanda vincitore solo se significativo
	if analysis.IsSignificant && maxSignificantDiff > 0 {
		analysis.RecommendedWinner = bestVariantID
	}

	log.Info().
		Str("experiment_id", exp.ID.String()).
		Bool("significant", analysis.IsSignificant).
		Float64("p_value", analysis.PValue).
		Str("recommended_winner", analysis.RecommendedWinner).
		Float64("effect_size", analysis.EffectSize).
		Msg("Completed experiment analysis")

	return analysis
}

// ChiSquareTest esegue il test chi-quadrato per success rate
func (sa *StatisticalAnalyzer) ChiSquareTest(statsA, statsB *VariantStatistics, idA, idB string) VariantComparison {
	comparison := VariantComparison{
		VariantA: idA,
		VariantB: idB,
		Metric:   "success_rate",
	}

	// Calcola differenza percentuale
	if statsA.SuccessRate > 0 {
		comparison.DiffPercent = ((statsB.SuccessRate - statsA.SuccessRate) / statsA.SuccessRate) * 100
	}

	// Determina variante migliore
	if statsB.SuccessRate > statsA.SuccessRate {
		comparison.BetterVariant = idB
	} else {
		comparison.BetterVariant = idA
	}

	// Verifica sample size minimo
	if statsA.Requests < 30 || statsB.Requests < 30 {
		comparison.PValue = 1.0
		comparison.IsSignificant = false
		return comparison
	}

	// Calcola chi-quadrato
	totalA := float64(statsA.Requests)
	totalB := float64(statsB.Requests)
	successA := float64(statsA.Successes)
	successB := float64(statsB.Successes)

	// Tabella di contingenza 2x2
	observedSuccessA := successA
	observedSuccessB := successB
	observedFailA := totalA - successA
	observedFailB := totalB - successB

	// Expected values
	totalSuccess := successA + successB
	totalFail := (totalA - successA) + (totalB - successB)
	total := totalA + totalB

	expectedSuccessA := (totalA * totalSuccess) / total
	expectedSuccessB := (totalB * totalSuccess) / total
	expectedFailA := (totalA * totalFail) / total
	expectedFailB := (totalB * totalFail) / total

	// Chi-square statistic
	chiSquare := 0.0
	if expectedSuccessA > 0 {
		chiSquare += math.Pow(observedSuccessA-expectedSuccessA, 2) / expectedSuccessA
	}
	if expectedSuccessB > 0 {
		chiSquare += math.Pow(observedSuccessB-expectedSuccessB, 2) / expectedSuccessB
	}
	if expectedFailA > 0 {
		chiSquare += math.Pow(observedFailA-expectedFailA, 2) / expectedFailA
	}
	if expectedFailB > 0 {
		chiSquare += math.Pow(observedFailB-expectedFailB, 2) / expectedFailB
	}

	// Degrees of freedom = 1 per tabella 2x2
	// P-value approssimato (chi-square con 1 df)
	comparison.PValue = sa.chiSquarePValue(chiSquare, 1)
	comparison.IsSignificant = comparison.PValue < (1.0 - sa.confidenceLevel)

	return comparison
}

// TTest esegue il t-test per metriche continue (latency, cost)
func (sa *StatisticalAnalyzer) TTest(statsA, statsB *VariantStatistics, idA, idB, metric string) VariantComparison {
	comparison := VariantComparison{
		VariantA: idA,
		VariantB: idB,
		Metric:   metric,
	}

	var meanA, meanB, stdA, stdB float64
	var nA, nB int64

	switch metric {
	case "latency":
		meanA = statsA.AvgLatencyMs
		meanB = statsB.AvgLatencyMs
		nA = statsA.Successes // Solo successi hanno latency
		nB = statsB.Successes
		stdA = sa.calculateStdDev(statsA.LatencySamples, meanA)
		stdB = sa.calculateStdDev(statsB.LatencySamples, meanB)

		// Per latency, più basso è meglio
		if meanB < meanA {
			comparison.BetterVariant = idB
		} else {
			comparison.BetterVariant = idA
		}
	case "cost":
		meanA = statsA.AvgCost
		meanB = statsB.AvgCost
		nA = statsA.Requests
		nB = statsB.Requests
		// Approssima std dev dal 20% della media
		stdA = meanA * 0.2
		stdB = meanB * 0.2

		// Per cost, più basso è meglio
		if meanB < meanA {
			comparison.BetterVariant = idB
		} else {
			comparison.BetterVariant = idA
		}
	default:
		comparison.PValue = 1.0
		return comparison
	}

	// Calcola differenza percentuale
	if meanA > 0 {
		comparison.DiffPercent = ((meanB - meanA) / meanA) * 100
	}

	// Verifica sample size minimo
	if nA < 30 || nB < 30 {
		comparison.PValue = 1.0
		comparison.IsSignificant = false
		return comparison
	}

	// Calcola t-statistic (Welch's t-test)
	numerator := meanA - meanB

	varA := stdA * stdA
	varB := stdB * stdB

	denominator := math.Sqrt((varA / float64(nA)) + (varB / float64(nB)))

	if denominator == 0 {
		comparison.PValue = 1.0
		comparison.IsSignificant = false
		return comparison
	}

	tStat := math.Abs(numerator / denominator)

	// Calcola degrees of freedom (Welch-Satterthwaite)
	df := sa.welchDF(nA, nB, varA, varB)

	// P-value approssimato
	comparison.PValue = sa.tTestPValue(tStat, df)
	comparison.IsSignificant = comparison.PValue < (1.0 - sa.confidenceLevel)

	return comparison
}

// calculateStdDev calcola la deviazione standard
func (sa *StatisticalAnalyzer) calculateStdDev(samples []int, mean float64) float64 {
	if len(samples) < 2 {
		return 0
	}

	var variance float64
	for _, sample := range samples {
		diff := float64(sample) - mean
		variance += diff * diff
	}
	variance /= float64(len(samples) - 1)

	return math.Sqrt(variance)
}

// welchDF calcola i gradi di libertà per Welch's t-test
func (sa *StatisticalAnalyzer) welchDF(n1, n2 int64, var1, var2 float64) int {
	s1 := var1 / float64(n1)
	s2 := var2 / float64(n2)

	numerator := math.Pow(s1+s2, 2)
	denominator := (math.Pow(s1, 2) / float64(n1-1)) + (math.Pow(s2, 2) / float64(n2-1))

	if denominator == 0 {
		return int(n1 + n2 - 2)
	}

	return int(numerator / denominator)
}

// chiSquarePValue calcola il p-value approssimato per chi-quadrato
func (sa *StatisticalAnalyzer) chiSquarePValue(chiSquare float64, df int) float64 {
	// Approssimazione semplificata usando tabella critica
	// Per df=1 e confidence 0.95, valore critico ~3.841
	criticalValues := map[float64]float64{
		0.90: 2.706,
		0.95: 3.841,
		0.99: 6.635,
	}

	critical := criticalValues[sa.confidenceLevel]
	if critical == 0 {
		critical = 3.841
	}

	if chiSquare > critical {
		// Approssimazione: più alto è chi-square, più basso è p-value
		// Per chi-square molto alto, p-value tende a 0
		return math.Max(0.001, critical/chiSquare*0.05)
	}

	// Approssimazione lineare per valori sotto il critico
	return 1.0 - (chiSquare / critical * (1.0 - 0.05))
}

// tTestPValue calcola il p-value approssimato per t-test
func (sa *StatisticalAnalyzer) tTestPValue(tStat float64, df int) float64 {
	// Approssimazione semplificata usando distribuzione normale per df > 30
	if df > 30 {
		// Z-score approssimazione
		return sa.normalPValue(tStat)
	}

	// Per df piccoli, usa tabella critica t-distribution
	criticalT := sa.tCritical(df, sa.confidenceLevel)

	if tStat > criticalT {
		// Approssimazione per valori significativi
		return math.Max(0.001, criticalT/tStat*(1.0-sa.confidenceLevel))
	}

	// Approssimazione lineare
	return 1.0 - (tStat / criticalT * (1.0 - (1.0-sa.confidenceLevel)))
}

// tCritical ritorna il valore critico t per un dato df e confidence level
func (sa *StatisticalAnalyzer) tCritical(df int, confidence float64) float64 {
	// Tabella semplificata dei valori critici t (two-tailed, alpha=0.05)
	criticalValues := map[int]float64{
		10: 2.228,
		20: 2.086,
		30: 2.042,
		60: 2.000,
		120: 1.980,
	}

	// Trova valore più vicino
	if df >= 120 {
		return 1.960
	}

	for k, v := range criticalValues {
		if df <= k {
			return v
		}
	}

	return 2.0
}

// normalPValue calcola p-value da distribuzione normale standard
func (sa *StatisticalAnalyzer) normalPValue(z float64) float64 {
	// Approssimazione usando funzione di errore
	z = math.Abs(z)

	// Per z > 3, p-value molto piccolo
	if z > 3 {
		return 0.001
	}

	// Approssimazione polinomiale semplificata
	// P(Z > z) ≈ e^(-z²/2) / (z√(2π))
	pval := math.Exp(-z*z/2) / (z * math.Sqrt(2*math.Pi))

	// Two-tailed
	return math.Min(1.0, 2*pval)
}

// CalculateCohenD calcola Cohen's d (effect size)
func (sa *StatisticalAnalyzer) CalculateCohenD(statsA, statsB *VariantStatistics, metric string) float64 {
	var meanA, meanB, stdA, stdB float64

	switch metric {
	case "success_rate":
		meanA = statsA.SuccessRate
		meanB = statsB.SuccessRate
		// Std dev per proporzioni: sqrt(p(1-p))
		stdA = math.Sqrt(meanA * (1 - meanA))
		stdB = math.Sqrt(meanB * (1 - meanB))
	case "latency":
		meanA = statsA.AvgLatencyMs
		meanB = statsB.AvgLatencyMs
		stdA = sa.calculateStdDev(statsA.LatencySamples, meanA)
		stdB = sa.calculateStdDev(statsB.LatencySamples, meanB)
	case "cost":
		meanA = statsA.AvgCost
		meanB = statsB.AvgCost
		stdA = meanA * 0.2
		stdB = meanB * 0.2
	default:
		return 0
	}

	// Pooled standard deviation
	pooledStd := math.Sqrt((stdA*stdA + stdB*stdB) / 2)

	if pooledStd == 0 {
		return 0
	}

	cohenD := (meanB - meanA) / pooledStd
	return math.Abs(cohenD)
}

// CalculateConfidenceInterval calcola l'intervallo di confidenza per una metrica
func (sa *StatisticalAnalyzer) CalculateConfidenceInterval(stats *VariantStatistics, metric string) (float64, float64) {
	var mean, stdErr float64

	switch metric {
	case "success_rate":
		mean = stats.SuccessRate
		// Standard error per proporzione: sqrt(p(1-p)/n)
		stdErr = math.Sqrt(mean * (1 - mean) / float64(stats.Requests))
	case "latency":
		if stats.Successes == 0 {
			return 0, 0
		}
		mean = stats.AvgLatencyMs
		std := sa.calculateStdDev(stats.LatencySamples, mean)
		stdErr = std / math.Sqrt(float64(stats.Successes))
	case "cost":
		mean = stats.AvgCost
		std := mean * 0.2
		stdErr = std / math.Sqrt(float64(stats.Requests))
	default:
		return 0, 0
	}

	// Z-score per confidence level (1.96 per 95%)
	zScore := 1.96
	if sa.confidenceLevel == 0.99 {
		zScore = 2.576
	} else if sa.confidenceLevel == 0.90 {
		zScore = 1.645
	}

	margin := zScore * stdErr
	return mean - margin, mean + margin
}

// HasSufficientSampleSize verifica se l'esperimento ha un campione sufficiente
func (sa *StatisticalAnalyzer) HasSufficientSampleSize(exp *Experiment) bool {
	if exp.MinSampleSize <= 0 {
		exp.MinSampleSize = 100 // Default minimo
	}

	for _, stats := range exp.Results.VariantStats {
		if stats.Requests < int64(exp.MinSampleSize) {
			return false
		}
	}

	return true
}

// getMetricValue estrae il valore della metrica dalle statistiche
func (sa *StatisticalAnalyzer) getMetricValue(stats *VariantStatistics, metric string) float64 {
	switch metric {
	case "success_rate":
		return stats.SuccessRate
	case "latency":
		return stats.AvgLatencyMs
	case "cost":
		return stats.AvgCost
	case "satisfaction":
		return stats.SatisfactionScore
	default:
		return stats.SuccessRate
	}
}

// RankVariants ordina le varianti in base alla metrica target
func (sa *StatisticalAnalyzer) RankVariants(exp *Experiment) []RankedVariant {
	rankings := make([]RankedVariant, 0, len(exp.Variants))

	for _, variant := range exp.Variants {
		stats, ok := exp.Results.VariantStats[variant.ID]
		if !ok {
			continue
		}

		metricValue := sa.getMetricValue(stats, exp.TargetMetric)
		lowerCI, upperCI := sa.CalculateConfidenceInterval(stats, exp.TargetMetric)

		rankings = append(rankings, RankedVariant{
			VariantID:   variant.ID,
			VariantName: variant.Name,
			MetricValue: metricValue,
			LowerCI:     lowerCI,
			UpperCI:     upperCI,
			SampleSize:  stats.Requests,
		})
	}

	// Ordina in base alla metrica
	isLowerBetter := exp.TargetMetric == "latency" || exp.TargetMetric == "cost"

	sort.Slice(rankings, func(i, j int) bool {
		if isLowerBetter {
			return rankings[i].MetricValue < rankings[j].MetricValue
		}
		return rankings[i].MetricValue > rankings[j].MetricValue
	})

	return rankings
}

// RankedVariant rappresenta una variante con il suo ranking
type RankedVariant struct {
	VariantID   string
	VariantName string
	MetricValue float64
	LowerCI     float64
	UpperCI     float64
	SampleSize  int64
}
