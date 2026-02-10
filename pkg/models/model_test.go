package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestModel_BeforeCreate(t *testing.T) {
	tests := []struct {
		name    string
		model   *Model
		wantErr bool
	}{
		{
			name: "generates UUID if nil",
			model: &Model{
				Name:     "test-model",
				Modality: ModalityChat,
			},
			wantErr: false,
		},
		{
			name: "keeps existing UUID",
			model: &Model{
				ID:       uuid.New(),
				Name:     "test-model",
				Modality: ModalityChat,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalID := tt.model.ID
			err := tt.model.BeforeCreate()

			if (err != nil) != tt.wantErr {
				t.Errorf("BeforeCreate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.model.ID == uuid.Nil {
				t.Error("ID should not be nil after BeforeCreate()")
			}

			if originalID != uuid.Nil && tt.model.ID != originalID {
				t.Error("Existing ID should not be changed")
			}
		})
	}
}

func TestModel_IsFree(t *testing.T) {
	tests := []struct {
		name  string
		model *Model
		want  bool
	}{
		{
			name: "free model",
			model: &Model{
				InputPricePer1k:  0.0,
				OutputPricePer1k: 0.0,
			},
			want: true,
		},
		{
			name: "paid model - input cost",
			model: &Model{
				InputPricePer1k:  0.001,
				OutputPricePer1k: 0.0,
			},
			want: false,
		},
		{
			name: "paid model - output cost",
			model: &Model{
				InputPricePer1k:  0.0,
				OutputPricePer1k: 0.002,
			},
			want: false,
		},
		{
			name: "paid model - both costs",
			model: &Model{
				InputPricePer1k:  0.001,
				OutputPricePer1k: 0.002,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.model.IsFree(); got != tt.want {
				t.Errorf("IsFree() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModel_EstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		model        *Model
		inputTokens  int
		outputTokens int
		want         float64
	}{
		{
			name: "free model",
			model: &Model{
				InputPricePer1k:  0.0,
				OutputPricePer1k: 0.0,
			},
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.0,
		},
		{
			name: "GPT-4 pricing",
			model: &Model{
				InputPricePer1k:  0.03,
				OutputPricePer1k: 0.06,
			},
			inputTokens:  1000,
			outputTokens: 500,
			want:         0.06, // 0.03 + 0.03
		},
		{
			name: "Claude pricing",
			model: &Model{
				InputPricePer1k:  0.008,
				OutputPricePer1k: 0.024,
			},
			inputTokens:  2000,
			outputTokens: 1000,
			want:         0.040, // 0.016 + 0.024
		},
		{
			name: "small request",
			model: &Model{
				InputPricePer1k:  0.001,
				OutputPricePer1k: 0.002,
			},
			inputTokens:  100,
			outputTokens: 50,
			want:         0.0002, // 0.0001 + 0.0001
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.EstimateCost(tt.inputTokens, tt.outputTokens)
			if !floatEqual(got, tt.want, 0.000001) {
				t.Errorf("EstimateCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModality_Constants(t *testing.T) {
	tests := []struct {
		name     string
		modality Modality
		expected string
	}{
		{"chat modality", ModalityChat, "chat"},
		{"completion modality", ModalityCompletion, "completion"},
		{"embedding modality", ModalityEmbedding, "embedding"},
		{"audio modality", ModalityAudio, "audio"},
		{"video modality", ModalityVideo, "video"},
		{"image modality", ModalityImage, "image"},
		{"realtime modality", ModalityRealtime, "realtime"},
		{"multimodal modality", ModalityMultimodal, "multimodal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.modality) != tt.expected {
				t.Errorf("Modality = %v, want %v", tt.modality, tt.expected)
			}
		})
	}
}

func TestModel_TableName(t *testing.T) {
	m := Model{}
	if got := m.TableName(); got != "models" {
		t.Errorf("TableName() = %v, want models", got)
	}
}

// Helper function for floating point comparison
func floatEqual(a, b, epsilon float64) bool {
	if a-b < epsilon && b-a < epsilon {
		return true
	}
	return false
}

// Benchmark tests
func BenchmarkModel_IsFree(b *testing.B) {
	model := &Model{
		InputPricePer1k:  0.001,
		OutputPricePer1k: 0.002,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.IsFree()
	}
}

func BenchmarkModel_EstimateCost(b *testing.B) {
	model := &Model{
		InputPricePer1k:  0.03,
		OutputPricePer1k: 0.06,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.EstimateCost(1000, 500)
	}
}
