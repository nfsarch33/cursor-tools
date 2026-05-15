package agentrace

import (
	"math"
)

type TrendType string

const (
	TrendConverging   TrendType = "converging"
	TrendDiverging    TrendType = "diverging"
	TrendPlateau      TrendType = "plateau"
	TrendInsufficient TrendType = "insufficient_data"
)

type ConvergenceResult struct {
	Trend      TrendType `json:"trend"`
	Slope      float64   `json:"slope"`
	StdDev     float64   `json:"std_dev"`
	Mean       float64   `json:"mean"`
	WindowSize int       `json:"window_size"`
	Confidence float64   `json:"confidence"`
}

type ConvergenceDetector struct {
	plateauThreshold float64
	minDataPoints    int
}

func NewConvergenceDetector(plateauThreshold float64, minDataPoints int) *ConvergenceDetector {
	if minDataPoints < 3 {
		minDataPoints = 3
	}
	return &ConvergenceDetector{
		plateauThreshold: plateauThreshold,
		minDataPoints:    minDataPoints,
	}
}

func (cd *ConvergenceDetector) Detect(values []float64) ConvergenceResult {
	n := len(values)
	if n < cd.minDataPoints {
		return ConvergenceResult{
			Trend:      TrendInsufficient,
			WindowSize: n,
		}
	}

	mean := Mean(values)
	stddev := StdDev(values, mean)
	slope := LinearSlope(values)

	cv := 0.0
	if mean != 0 {
		cv = stddev / math.Abs(mean)
	}

	result := ConvergenceResult{
		Slope:      slope,
		StdDev:     stddev,
		Mean:       mean,
		WindowSize: n,
	}

	if cv < cd.plateauThreshold {
		result.Trend = TrendPlateau
		result.Confidence = 1.0 - cv/cd.plateauThreshold
	} else if slope > 0 {
		result.Trend = TrendConverging
		result.Confidence = math.Min(1.0, slope/stddev)
	} else {
		result.Trend = TrendDiverging
		result.Confidence = math.Min(1.0, math.Abs(slope)/stddev)
	}

	return result
}

func (cd *ConvergenceDetector) DetectFromEvents(events []Event, metricName string) ConvergenceResult {
	var values []float64

	for _, e := range events {
		if e.Type != EventMetric {
			continue
		}
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			continue
		}
		name, _ := payload["name"].(string)
		if name != metricName {
			continue
		}
		val, ok := payload["value"].(float64)
		if ok {
			values = append(values, val)
		}
	}

	return cd.Detect(values)
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func StdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(values)-1))
}

func LinearSlope(values []float64) float64 {
	n := float64(len(values))
	if n < 2 {
		return 0
	}

	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}
