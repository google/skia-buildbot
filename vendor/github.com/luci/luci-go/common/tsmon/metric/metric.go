// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package metric is the API for defining metrics and updating their values.
//
// When you define a metric you must also define the names and types of any
// fields on that metric.  It is an error to define two metrics with the same
// name (this will cause a panic).
//
// Example:
//   var (
//     Requests = metric.NewCounter("myapp/requests", field.String("status"))
//   )
//   ...
//   func handleRequest() {
//     if success {
//       Requests.Add(1, "success")
//     } else {
//       Requests.Add(1, "failure")
//     }
//   }
package metric

import (
	"time"

	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

// Int is a non-cumulative integer gauge metric.
type Int interface {
	types.Metric

	Get(c context.Context, fieldVals ...interface{}) (int64, error)
	Set(c context.Context, v int64, fieldVals ...interface{}) error
}

// Counter is a cumulative integer metric.
type Counter interface {
	Int
	Add(c context.Context, n int64, fieldVals ...interface{}) error
}

// Float is a non-cumulative floating-point gauge metric.
type Float interface {
	types.Metric

	Get(c context.Context, fieldVals ...interface{}) (float64, error)
	Set(c context.Context, v float64, fieldVals ...interface{}) error
}

// FloatCounter is a cumulative floating-point metric.
type FloatCounter interface {
	Float
	Add(c context.Context, n float64, fieldVals ...interface{}) error
}

// String is a string-valued metric.
type String interface {
	types.Metric

	Get(c context.Context, fieldVals ...interface{}) (string, error)
	Set(c context.Context, v string, fieldVals ...interface{}) error
}

// Bool is a boolean-valued metric.
type Bool interface {
	types.Metric

	Get(c context.Context, fieldVals ...interface{}) (bool, error)
	Set(c context.Context, v bool, fieldVals ...interface{}) error
}

// NonCumulativeDistribution is a non-cumulative-distribution-valued metric.
type NonCumulativeDistribution interface {
	types.DistributionMetric

	Get(c context.Context, fieldVals ...interface{}) (*distribution.Distribution, error)
	Set(c context.Context, d *distribution.Distribution, fieldVals ...interface{}) error
}

// CumulativeDistribution is a cumulative-distribution-valued metric.
type CumulativeDistribution interface {
	NonCumulativeDistribution
	Add(c context.Context, v float64, fieldVals ...interface{}) error
}

// CallbackInt is a non-cumulative integer gauge metric whose values can only be
// set from a callback.  Use tsmon.RegisterCallback to add a callback to set the
// metric's values.
type CallbackInt interface {
	types.Metric

	Set(ctx context.Context, v int64, fieldVals ...interface{}) error
}

// CallbackFloat is a non-cumulative floating-point gauge metric whose values
// can only be set from a callback.  Use tsmon.RegisterCallback to add a
// callback to set the metric's values.
type CallbackFloat interface {
	types.Metric

	Set(ctx context.Context, v float64, fieldVals ...interface{}) error
}

// CallbackString is a string-valued metric whose values can only be set from a
// callback.  Use tsmon.RegisterCallback to add a callback to set the metric's
// values.
type CallbackString interface {
	types.Metric

	Set(ctx context.Context, v string, fieldVals ...interface{}) error
}

// CallbackBool is a boolean-valued metric whose values can only be set from a
// callback.  Use tsmon.RegisterCallback to add a callback to set the metric's
// values.
type CallbackBool interface {
	types.Metric

	Set(ctx context.Context, v bool, fieldVals ...interface{}) error
}

// CallbackDistribution is a non-cumulative-distribution-valued  metric whose
// values can only be set from a callback.  Use tsmon.RegisterCallback to add a
// callback to set the metric's values.
type CallbackDistribution interface {
	types.DistributionMetric

	Set(ctx context.Context, d *distribution.Distribution, fieldVals ...interface{}) error
}

// NewInt returns a new non-cumulative integer gauge metric.  This will panic if
// another metric already exists with this name.
func NewInt(name string, description string, metadata types.MetricMetadata, fields ...field.Field) Int {
	return NewIntIn(context.Background(), name, description, metadata, fields...)
}

// NewIntIn is like NewInt but registers in a given context.
func NewIntIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) Int {
	m := &intMetric{metric{
		MetricInfo: types.MetricInfo{
			Name:        name,
			Description: description,
			Fields:      fields,
			ValueType:   types.NonCumulativeIntType,
		},
		MetricMetadata: metadata,
	}}
	tsmon.Register(c, m)
	return m
}

// NewCounter returns a new cumulative integer metric.  This will panic if
// another metric already exists with this name.
func NewCounter(name string, description string, metadata types.MetricMetadata, fields ...field.Field) Counter {
	return NewCounterIn(context.Background(), name, description, metadata, fields...)
}

// NewCounterIn is like NewCounter but registers in a given context.
func NewCounterIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) Counter {
	m := &counter{intMetric{metric{MetricInfo: types.MetricInfo{
		Name:        name,
		Description: description,
		Fields:      fields,
		ValueType:   types.CumulativeIntType,
	}}}}
	tsmon.Register(c, m)
	return m
}

// NewFloat returns a new non-cumulative floating-point gauge metric.  This will
// panic if another metric already exists with this name.
func NewFloat(name string, description string, metadata types.MetricMetadata, fields ...field.Field) Float {
	return NewFloatIn(context.Background(), name, description, metadata, fields...)
}

// NewFloatIn is like NewFloat but registers in a given context.
func NewFloatIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) Float {
	m := &floatMetric{metric{MetricInfo: types.MetricInfo{
		Name:        name,
		Description: description,
		Fields:      fields,
		ValueType:   types.NonCumulativeFloatType,
	}}}
	tsmon.Register(c, m)
	return m
}

// NewFloatCounter returns a new cumulative floating-point metric.  This will
// panic if another metric already exists with this name.
func NewFloatCounter(name string, description string, metadata types.MetricMetadata, fields ...field.Field) FloatCounter {
	return NewFloatCounterIn(context.Background(), name, description, metadata, fields...)
}

// NewFloatCounterIn is like NewFloatCounter but registers in a given context.
func NewFloatCounterIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) FloatCounter {
	m := &floatCounter{floatMetric{metric{MetricInfo: types.MetricInfo{
		Name:        name,
		Description: description,
		Fields:      fields,
		ValueType:   types.CumulativeFloatType,
	}}}}
	tsmon.Register(c, m)
	return m
}

// NewString returns a new string-valued metric.  This will panic if another
// metric already exists with this name.
func NewString(name string, description string, metadata types.MetricMetadata, fields ...field.Field) String {
	return NewStringIn(context.Background(), name, description, metadata, fields...)
}

// NewStringIn is like NewString but registers in a given context.
func NewStringIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) String {
	m := &stringMetric{metric{MetricInfo: types.MetricInfo{
		Name:        name,
		Description: description,
		Fields:      fields,
		ValueType:   types.StringType,
	}}}
	tsmon.Register(c, m)
	return m
}

// NewBool returns a new bool-valued metric.  This will panic if another
// metric already exists with this name.
func NewBool(name string, description string, metadata types.MetricMetadata, fields ...field.Field) Bool {
	return NewBoolIn(context.Background(), name, description, metadata, fields...)
}

// NewBoolIn is like NewBool but registers in a given context.
func NewBoolIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) Bool {
	m := &boolMetric{metric{MetricInfo: types.MetricInfo{
		Name:        name,
		Description: description,
		Fields:      fields,
		ValueType:   types.BoolType,
	}}}
	tsmon.Register(c, m)
	return m
}

// NewCumulativeDistribution returns a new cumulative-distribution-valued
// metric.  This will panic if another metric already exists with this name.
func NewCumulativeDistribution(name string, description string, metadata types.MetricMetadata, bucketer *distribution.Bucketer, fields ...field.Field) CumulativeDistribution {
	return NewCumulativeDistributionIn(context.Background(), name, description, metadata, bucketer, fields...)
}

// NewCumulativeDistributionIn is like NewCumulativeDistribution but registers in a given context.
func NewCumulativeDistributionIn(c context.Context, name string, description string, metadata types.MetricMetadata, bucketer *distribution.Bucketer, fields ...field.Field) CumulativeDistribution {
	m := &cumulativeDistributionMetric{
		nonCumulativeDistributionMetric{
			metric: metric{MetricInfo: types.MetricInfo{
				Name:        name,
				Description: description,
				Fields:      fields,
				ValueType:   types.CumulativeDistributionType,
			}},
			bucketer: bucketer,
		},
	}
	tsmon.Register(c, m)
	return m
}

// NewNonCumulativeDistribution returns a new non-cumulative-distribution-valued
// metric.  This will panic if another metric already exists with this name.
func NewNonCumulativeDistribution(name string, description string, metadata types.MetricMetadata, bucketer *distribution.Bucketer, fields ...field.Field) NonCumulativeDistribution {
	return NewNonCumulativeDistributionIn(context.Background(), name, description, metadata, bucketer, fields...)
}

// NewNonCumulativeDistributionIn is like NewNonCumulativeDistribution but registers in a given context.
func NewNonCumulativeDistributionIn(c context.Context, name string, description string, metadata types.MetricMetadata, bucketer *distribution.Bucketer, fields ...field.Field) NonCumulativeDistribution {
	m := &nonCumulativeDistributionMetric{
		metric: metric{MetricInfo: types.MetricInfo{
			Name:        name,
			Description: description,
			Fields:      fields,
			ValueType:   types.NonCumulativeDistributionType,
		}},
		bucketer: bucketer,
	}
	tsmon.Register(c, m)
	return m
}

// NewCallbackInt returns a new integer metric whose value is populated by a
// callback at collection time.
func NewCallbackInt(name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackInt {
	return NewInt(name, description, metadata, fields...)
}

// NewCallbackFloat returns a new float metric whose value is populated by a
// callback at collection time.
func NewCallbackFloat(name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackFloat {
	return NewFloat(name, description, metadata, fields...)
}

// NewCallbackString returns a new string metric whose value is populated by a
// callback at collection time.
func NewCallbackString(name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackString {
	return NewString(name, description, metadata, fields...)
}

// NewCallbackBool returns a new bool metric whose value is populated by a
// callback at collection time.
func NewCallbackBool(name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackBool {
	return NewBool(name, description, metadata, fields...)
}

// NewCallbackDistribution returns a new distribution metric whose value is
// populated by a callback at collection time.
func NewCallbackDistribution(name string, description string, metadata types.MetricMetadata, bucketer *distribution.Bucketer, fields ...field.Field) CallbackDistribution {
	return NewNonCumulativeDistribution(name, description, metadata, bucketer, fields...)
}

// NewCallbackIntIn is like NewCallbackInt but registers in a given context.
func NewCallbackIntIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackInt {
	return NewIntIn(c, name, description, metadata, fields...)
}

// NewCallbackFloatIn is like NewCallbackFloat but registers in a given context.
func NewCallbackFloatIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackFloat {
	return NewFloatIn(c, name, description, metadata, fields...)
}

// NewCallbackStringIn is like NewCallbackString but registers in a given context.
func NewCallbackStringIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackString {
	return NewStringIn(c, name, description, metadata, fields...)
}

// NewCallbackBoolIn is like NewCallbackBool but registers in a given context.
func NewCallbackBoolIn(c context.Context, name string, description string, metadata types.MetricMetadata, fields ...field.Field) CallbackBool {
	return NewBoolIn(c, name, description, metadata, fields...)
}

// NewCallbackDistributionIn is like NewCallbackDistribution but registers in a given context.
func NewCallbackDistributionIn(c context.Context, name string, description string, metadata types.MetricMetadata, bucketer *distribution.Bucketer, fields ...field.Field) CallbackDistribution {
	return NewNonCumulativeDistributionIn(c, name, description, metadata, bucketer, fields...)
}

// genericGet is a convenience function that tries to get a metric value from
// the store and returns the zero value if it didn't exist.
func (m *metric) genericGet(zero interface{}, c context.Context, fieldVals []interface{}) (interface{}, error) {
	switch ret, err := tsmon.Store(c).Get(c, m, m.fixedResetTime, fieldVals); {
	case err != nil:
		return zero, err
	case ret == nil:
		return zero, nil
	default:
		return ret, nil
	}
}

type metric struct {
	types.MetricInfo
	types.MetricMetadata
	fixedResetTime time.Time
}

func (m *metric) Info() types.MetricInfo         { return m.MetricInfo }
func (m *metric) Metadata() types.MetricMetadata { return m.MetricMetadata }
func (m *metric) SetFixedResetTime(t time.Time)  { m.fixedResetTime = t }

type intMetric struct{ metric }

func (m *intMetric) Get(c context.Context, fieldVals ...interface{}) (int64, error) {
	ret, err := m.genericGet(int64(0), c, fieldVals)
	return ret.(int64), err
}

func (m *intMetric) Set(c context.Context, v int64, fieldVals ...interface{}) error {
	return tsmon.Store(c).Set(c, m, m.fixedResetTime, fieldVals, v)
}

type counter struct{ intMetric }

func (m *counter) Add(c context.Context, n int64, fieldVals ...interface{}) error {
	return tsmon.Store(c).Incr(c, m, m.fixedResetTime, fieldVals, n)
}

type floatMetric struct{ metric }

func (m *floatMetric) Get(c context.Context, fieldVals ...interface{}) (float64, error) {
	ret, err := m.genericGet(float64(0.0), c, fieldVals)
	return ret.(float64), err
}

func (m *floatMetric) Set(c context.Context, v float64, fieldVals ...interface{}) error {
	return tsmon.Store(c).Set(c, m, m.fixedResetTime, fieldVals, v)
}

type floatCounter struct{ floatMetric }

func (m *floatCounter) Add(c context.Context, n float64, fieldVals ...interface{}) error {
	return tsmon.Store(c).Incr(c, m, m.fixedResetTime, fieldVals, n)
}

type stringMetric struct{ metric }

func (m *stringMetric) Get(c context.Context, fieldVals ...interface{}) (string, error) {
	ret, err := m.genericGet("", c, fieldVals)
	return ret.(string), err
}

func (m *stringMetric) Set(c context.Context, v string, fieldVals ...interface{}) error {
	return tsmon.Store(c).Set(c, m, m.fixedResetTime, fieldVals, v)
}

type boolMetric struct{ metric }

func (m *boolMetric) Get(c context.Context, fieldVals ...interface{}) (bool, error) {
	ret, err := m.genericGet(false, c, fieldVals)
	return ret.(bool), err
}

func (m *boolMetric) Set(c context.Context, v bool, fieldVals ...interface{}) error {
	return tsmon.Store(c).Set(c, m, m.fixedResetTime, fieldVals, v)
}

type nonCumulativeDistributionMetric struct {
	metric
	bucketer *distribution.Bucketer
}

func (m *nonCumulativeDistributionMetric) Bucketer() *distribution.Bucketer { return m.bucketer }

func (m *nonCumulativeDistributionMetric) Get(c context.Context, fieldVals ...interface{}) (*distribution.Distribution, error) {
	ret, err := m.genericGet((*distribution.Distribution)(nil), c, fieldVals)
	return ret.(*distribution.Distribution), err
}

func (m *nonCumulativeDistributionMetric) Set(c context.Context, v *distribution.Distribution, fieldVals ...interface{}) error {
	return tsmon.Store(c).Set(c, m, m.fixedResetTime, fieldVals, v)
}

type cumulativeDistributionMetric struct {
	nonCumulativeDistributionMetric
}

func (m *cumulativeDistributionMetric) Add(c context.Context, v float64, fieldVals ...interface{}) error {
	return tsmon.Store(c).Incr(c, m, m.fixedResetTime, fieldVals, v)
}
