package noop

// Float64SummaryMetric implements metrics2.Float64SummaryMetric as a no-op
type Float64SummaryMetric struct{}

func (f Float64SummaryMetric) Observe(_ float64) {}
