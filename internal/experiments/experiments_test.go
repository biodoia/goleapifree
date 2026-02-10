package experiments

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUserBucketing(t *testing.T) {
	exp := &Experiment{
		ID: uuid.New(),
		Variants: []Variant{
			{ID: "control", Name: "Control", IsControl: true},
			{ID: "variant_a", Name: "Variant A"},
			{ID: "variant_b", Name: "Variant B"},
		},
		TrafficSplit: map[string]int{
			"control":   34,
			"variant_a": 33,
			"variant_b": 33,
		},
	}

	bucketing := NewUserBucketing(exp)

	// Test che lo stesso utente riceve sempre la stessa variante
	userID := uuid.New()
	variant1 := bucketing.AssignVariant(userID)
	variant2 := bucketing.AssignVariant(userID)

	assert.Equal(t, variant1.ID, variant2.ID, "User should get consistent variant")

	// Test distribuzione su molti utenti
	distribution := make(map[string]int)
	totalUsers := 10000

	for i := 0; i < totalUsers; i++ {
		userID := uuid.New()
		variant := bucketing.AssignVariant(userID)
		distribution[variant.ID]++
	}

	// Verifica che la distribuzione sia circa 33-34% per variante
	for variantID, count := range distribution {
		percentage := float64(count) / float64(totalUsers) * 100
		t.Logf("Variant %s: %.2f%% (%d users)", variantID, percentage, count)

		// Verifica che sia entro 5% dal target
		expected := float64(exp.TrafficSplit[variantID])
		assert.InDelta(t, expected, percentage, 5.0, "Distribution should be close to target")
	}
}

func TestExperimentFilters(t *testing.T) {
	exp := &Experiment{
		ID: uuid.New(),
		Variants: []Variant{
			{ID: "control", Name: "Control"},
			{ID: "variant_a", Name: "Variant A"},
		},
		TrafficSplit: map[string]int{
			"control":   50,
			"variant_a": 50,
		},
		Filters: ExperimentFilters{
			MinTokens: 100,
			MaxTokens: 1000,
			UserSegments: []string{"premium"},
		},
	}

	bucketing := NewUserBucketing(exp)

	// Test richiesta che passa i filtri
	validReq := &ExperimentRequest{
		UserID:       uuid.New(),
		UserSegment:  "premium",
		InputTokens:  50,
		OutputTokens: 100,
	}
	assert.True(t, bucketing.ShouldIncludeRequest(validReq), "Valid request should pass filters")

	// Test richiesta con troppi pochi token
	tooFewTokens := &ExperimentRequest{
		UserID:       uuid.New(),
		UserSegment:  "premium",
		InputTokens:  20,
		OutputTokens: 30,
	}
	assert.False(t, bucketing.ShouldIncludeRequest(tooFewTokens), "Request with too few tokens should fail")

	// Test richiesta con troppi token
	tooManyTokens := &ExperimentRequest{
		UserID:       uuid.New(),
		UserSegment:  "premium",
		InputTokens:  800,
		OutputTokens: 400,
	}
	assert.False(t, bucketing.ShouldIncludeRequest(tooManyTokens), "Request with too many tokens should fail")

	// Test richiesta con segment sbagliato
	wrongSegment := &ExperimentRequest{
		UserID:       uuid.New(),
		UserSegment:  "free",
		InputTokens:  50,
		OutputTokens: 100,
	}
	assert.False(t, bucketing.ShouldIncludeRequest(wrongSegment), "Request with wrong segment should fail")
}

func TestFeatureFlagRollout(t *testing.T) {
	flag := &FeatureFlag{
		ID:                uuid.New(),
		Name:              "test_feature",
		Enabled:           true,
		RolloutPercentage: 50,
	}

	enabled := 0
	disabled := 0
	totalUsers := 10000

	for i := 0; i < totalUsers; i++ {
		userID := uuid.New()
		if flag.IsEnabledForUser(userID) {
			enabled++
		} else {
			disabled++
		}
	}

	enabledPct := float64(enabled) / float64(totalUsers) * 100
	t.Logf("Enabled: %.2f%% (%d users)", enabledPct, enabled)

	// Verifica che sia circa 50%
	assert.InDelta(t, 50.0, enabledPct, 5.0, "Rollout percentage should be close to 50%")
}

func TestFeatureFlagWhitelist(t *testing.T) {
	whitelistedUser := uuid.New()
	regularUser := uuid.New()

	flag := &FeatureFlag{
		ID:                uuid.New(),
		Name:              "beta_feature",
		Enabled:           true,
		RolloutPercentage: 0, // 0% rollout
		UserWhitelist:     []uuid.UUID{whitelistedUser},
	}

	// Whitelisted user dovrebbe essere abilitato
	assert.True(t, flag.IsEnabledForUser(whitelistedUser), "Whitelisted user should be enabled")

	// Regular user dovrebbe essere disabilitato
	assert.False(t, flag.IsEnabledForUser(regularUser), "Regular user should be disabled")
}

func TestFeatureFlagBlacklist(t *testing.T) {
	blacklistedUser := uuid.New()
	regularUser := uuid.New()

	flag := &FeatureFlag{
		ID:                uuid.New(),
		Name:              "new_feature",
		Enabled:           true,
		RolloutPercentage: 100, // 100% rollout
		UserBlacklist:     []uuid.UUID{blacklistedUser},
	}

	// Blacklisted user dovrebbe essere disabilitato
	assert.False(t, flag.IsEnabledForUser(blacklistedUser), "Blacklisted user should be disabled")

	// Regular user dovrebbe essere abilitato
	assert.True(t, flag.IsEnabledForUser(regularUser), "Regular user should be enabled")
}

func TestConsistentHashing(t *testing.T) {
	exp := &Experiment{
		ID: uuid.New(),
		Variants: []Variant{
			{ID: "control", Name: "Control"},
			{ID: "variant_a", Name: "Variant A"},
		},
		TrafficSplit: map[string]int{
			"control":   50,
			"variant_a": 50,
		},
	}

	bucketing := NewUserBucketing(exp)

	// Test che lo stesso utente riceve sempre lo stesso hash
	userID := uuid.New()
	hash1 := bucketing.consistentHash(userID)
	hash2 := bucketing.consistentHash(userID)

	assert.Equal(t, hash1, hash2, "Hash should be consistent for same user")

	// Test che utenti diversi ricevono hash diversi
	userID2 := uuid.New()
	hash3 := bucketing.consistentHash(userID2)

	assert.NotEqual(t, hash1, hash3, "Different users should get different hashes")
}

func BenchmarkUserBucketing(b *testing.B) {
	exp := &Experiment{
		ID: uuid.New(),
		Variants: []Variant{
			{ID: "control", Name: "Control"},
			{ID: "variant_a", Name: "Variant A"},
			{ID: "variant_b", Name: "Variant B"},
		},
		TrafficSplit: map[string]int{
			"control":   34,
			"variant_a": 33,
			"variant_b": 33,
		},
	}

	bucketing := NewUserBucketing(exp)
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucketing.AssignVariant(userID)
	}
}

func BenchmarkFeatureFlagCheck(b *testing.B) {
	flag := &FeatureFlag{
		ID:                uuid.New(),
		Name:              "test_feature",
		Enabled:           true,
		RolloutPercentage: 50,
	}

	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flag.IsEnabledForUser(userID)
	}
}
