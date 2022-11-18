package metrics2

/*
Metrics2 Package
================

Concepts
--------

The major difference between the old metrics format and the new is that, from
the app’s point of view, the old metrics were specified using a single metric
name.  In the new format, metrics are specified using a measurement name and a
map[string]string of tags.  Expect all instances of metrics types to require
both a measurement name and tags map instead of just the metric name.  When
using common.InitWithMetrics2, runtime stats including uptime are automatically
recorded, and the “host” and “app” tags are automatically set for all
measurements.  In many cases, it may be unnecessary to provide more tags, so
you can just pass nil.

Metrics Helpers
---------------
The metrics2 package provides a number of helper structs and functions which
should be your primary mode of interacting with metrics.

### Typed Metrics

Metrics2 provides a few typed metrics helpers which behave similarly to
go_metrics.GetOrRegisterGauge: once registered, the metric contains a single
value which is reported at regular intervals.  The value can be changed by
calling Update() on the Metric. The metrics are stored in a sort of registry
so that you don’t need to keep the object around and can just do:
metrics2.GetInt64Metric(metric, tags).Update(value).

### Counter

Counter in the metrics2 package behaves similarly to those from go-metrics
except that there is no GetOrRegisterCounter equivalent.  Instead, you should
call metrics2.GetCounter(name, tags), hold on to the returned struct instance
and call Inc(), Dec(), etc, on it as desired. Note that Counter requires a
name and not a measurement, because the measurement is always “counter”, and
the provided name is inserted as a tag.

### Liveness

Liveness in metrics2 behaves similarly to the old metrics liveness, except that
you provide a name and tags.  Call metrics2.NewLiveness() to start the liveness
timer, and call Reset() on the instance to reset it to zero.  Note that
Liveness requires a name and not a measurement, because the measurement is
always “liveness”, and the provided name is inserted as a tag.

### Timer

Timer in metrics2 behaves similarly to the old metrics timer, except that you
provide a name and tags instead of a metric.  Call metrics2.NewTimer(name,
tags) to start the timer and call Stop() on the instance to measure the
duration and report the duration. Timer does not behave like a Gauge, in that
it does not push values at regular intervals.  Instead, it only pushes a value
when Stop() is called.  Be aware of this when creating alerts based on timers,
since data points will not be evenly spaced and may not exist for a time
period.  Timer requires a name and not a measurement, because the measurement
is always “timer” and the provided name is inserted as a tag.

### FuncTimer

FuncTimer is a special Timer designed specifically for timing the duration of
functions.  It does not accept any parameters because it automatically fills in
the function name and package name in the tags.  Just do defer
metrics2.FuncTimer().Stop() at the beginning of the function.

*/
