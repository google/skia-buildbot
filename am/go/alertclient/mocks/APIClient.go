// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	incident "go.skia.org/infra/am/go/incident"

	silence "go.skia.org/infra/am/go/silence"
)

// APIClient is an autogenerated mock type for the APIClient type
type APIClient struct {
	mock.Mock
}

// GetAlerts provides a mock function with given fields:
func (_m *APIClient) GetAlerts() ([]incident.Incident, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetAlerts")
	}

	var r0 []incident.Incident
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]incident.Incident, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []incident.Incident); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]incident.Incident)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSilences provides a mock function with given fields:
func (_m *APIClient) GetSilences() ([]silence.Silence, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetSilences")
	}

	var r0 []silence.Silence
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]silence.Silence, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []silence.Silence); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]silence.Silence)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAPIClient creates a new instance of APIClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAPIClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *APIClient {
	mock := &APIClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
