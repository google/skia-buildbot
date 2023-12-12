// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	iam "cloud.google.com/go/iam"
	gopubsub "go.skia.org/infra/go/pubsub"

	mock "github.com/stretchr/testify/mock"

	pubsub "cloud.google.com/go/pubsub"
)

// Topic is an autogenerated mock type for the Topic type
type Topic struct {
	mock.Mock
}

// Config provides a mock function with given fields: ctx
func (_m *Topic) Config(ctx context.Context) (pubsub.TopicConfig, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Config")
	}

	var r0 pubsub.TopicConfig
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (pubsub.TopicConfig, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) pubsub.TopicConfig); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(pubsub.TopicConfig)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: ctx
func (_m *Topic) Delete(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Exists provides a mock function with given fields: ctx
func (_m *Topic) Exists(ctx context.Context) (bool, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Exists")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (bool, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) bool); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Flush provides a mock function with given fields:
func (_m *Topic) Flush() {
	_m.Called()
}

// IAM provides a mock function with given fields:
func (_m *Topic) IAM() *iam.Handle {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for IAM")
	}

	var r0 *iam.Handle
	if rf, ok := ret.Get(0).(func() *iam.Handle); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*iam.Handle)
		}
	}

	return r0
}

// ID provides a mock function with given fields:
func (_m *Topic) ID() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ID")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Publish provides a mock function with given fields: ctx, msg
func (_m *Topic) Publish(ctx context.Context, msg *pubsub.Message) gopubsub.PublishResult {
	ret := _m.Called(ctx, msg)

	if len(ret) == 0 {
		panic("no return value specified for Publish")
	}

	var r0 gopubsub.PublishResult
	if rf, ok := ret.Get(0).(func(context.Context, *pubsub.Message) gopubsub.PublishResult); ok {
		r0 = rf(ctx, msg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(gopubsub.PublishResult)
		}
	}

	return r0
}

// ResumePublish provides a mock function with given fields: orderingKey
func (_m *Topic) ResumePublish(orderingKey string) {
	_m.Called(orderingKey)
}

// Stop provides a mock function with given fields:
func (_m *Topic) Stop() {
	_m.Called()
}

// String provides a mock function with given fields:
func (_m *Topic) String() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for String")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Subscriptions provides a mock function with given fields: ctx
func (_m *Topic) Subscriptions(ctx context.Context) *pubsub.SubscriptionIterator {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Subscriptions")
	}

	var r0 *pubsub.SubscriptionIterator
	if rf, ok := ret.Get(0).(func(context.Context) *pubsub.SubscriptionIterator); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pubsub.SubscriptionIterator)
		}
	}

	return r0
}

// Update provides a mock function with given fields: ctx, cfg
func (_m *Topic) Update(ctx context.Context, cfg pubsub.TopicConfigToUpdate) (pubsub.TopicConfig, error) {
	ret := _m.Called(ctx, cfg)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 pubsub.TopicConfig
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, pubsub.TopicConfigToUpdate) (pubsub.TopicConfig, error)); ok {
		return rf(ctx, cfg)
	}
	if rf, ok := ret.Get(0).(func(context.Context, pubsub.TopicConfigToUpdate) pubsub.TopicConfig); ok {
		r0 = rf(ctx, cfg)
	} else {
		r0 = ret.Get(0).(pubsub.TopicConfig)
	}

	if rf, ok := ret.Get(1).(func(context.Context, pubsub.TopicConfigToUpdate) error); ok {
		r1 = rf(ctx, cfg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewTopic creates a new instance of Topic. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTopic(t interface {
	mock.TestingT
	Cleanup(func())
}) *Topic {
	mock := &Topic{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}