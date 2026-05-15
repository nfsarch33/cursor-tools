package agentrace

import (
	"math"
	"testing"
)

func TestConvergenceDetector_Plateau(t *testing.T) {
	cd := NewConvergenceDetector(0.05, 3)
	values := []float64{100, 100.1, 99.9, 100, 100.05}

	result := cd.Detect(values)
	if result.Trend != TrendPlateau {
		t.Errorf("Trend = %q, want plateau", result.Trend)
	}
	if result.Confidence < 0.5 {
		t.Errorf("Confidence = %f, want >= 0.5 for plateau", result.Confidence)
	}
}

func TestConvergenceDetector_Converging(t *testing.T) {
	cd := NewConvergenceDetector(0.05, 3)
	values := []float64{50, 60, 70, 80, 90, 100}

	result := cd.Detect(values)
	if result.Trend != TrendConverging {
		t.Errorf("Trend = %q, want converging", result.Trend)
	}
	if result.Slope <= 0 {
		t.Errorf("Slope = %f, want positive", result.Slope)
	}
}

func TestConvergenceDetector_Diverging(t *testing.T) {
	cd := NewConvergenceDetector(0.05, 3)
	values := []float64{100, 90, 80, 70, 60, 50}

	result := cd.Detect(values)
	if result.Trend != TrendDiverging {
		t.Errorf("Trend = %q, want diverging", result.Trend)
	}
	if result.Slope >= 0 {
		t.Errorf("Slope = %f, want negative", result.Slope)
	}
}

func TestConvergenceDetector_InsufficientData(t *testing.T) {
	cd := NewConvergenceDetector(0.05, 5)
	values := []float64{1, 2}

	result := cd.Detect(values)
	if result.Trend != TrendInsufficient {
		t.Errorf("Trend = %q, want insufficient_data", result.Trend)
	}
}

func TestMean(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"empty", []float64{}, 0},
		{"single", []float64{42}, 42},
		{"multiple", []float64{10, 20, 30}, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mean(tt.values)
			if got != tt.want {
				t.Errorf("Mean(%v) = %f, want %f", tt.values, got, tt.want)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	mean := Mean(values)
	sd := StdDev(values, mean)
	if math.Abs(sd-2.0) > 0.1 {
		t.Errorf("StdDev = %f, want ~2.0", sd)
	}
}

func TestStdDev_SingleValue(t *testing.T) {
	sd := StdDev([]float64{42}, 42)
	if sd != 0 {
		t.Errorf("StdDev of single value = %f, want 0", sd)
	}
}

func TestLinearSlope_Positive(t *testing.T) {
	values := []float64{0, 1, 2, 3, 4}
	slope := LinearSlope(values)
	if math.Abs(slope-1.0) > 0.001 {
		t.Errorf("LinearSlope = %f, want 1.0", slope)
	}
}

func TestLinearSlope_Negative(t *testing.T) {
	values := []float64{4, 3, 2, 1, 0}
	slope := LinearSlope(values)
	if math.Abs(slope-(-1.0)) > 0.001 {
		t.Errorf("LinearSlope = %f, want -1.0", slope)
	}
}

func TestLinearSlope_Flat(t *testing.T) {
	values := []float64{5, 5, 5, 5}
	slope := LinearSlope(values)
	if slope != 0 {
		t.Errorf("LinearSlope = %f, want 0", slope)
	}
}

func TestLinearSlope_TooFew(t *testing.T) {
	slope := LinearSlope([]float64{42})
	if slope != 0 {
		t.Errorf("LinearSlope(single) = %f, want 0", slope)
	}
}

func TestConvergenceDetector_MinDataPoints(t *testing.T) {
	cd := NewConvergenceDetector(0.05, 1)
	if cd.minDataPoints < 3 {
		t.Errorf("minDataPoints = %d, want >= 3 (enforced minimum)", cd.minDataPoints)
	}
}
