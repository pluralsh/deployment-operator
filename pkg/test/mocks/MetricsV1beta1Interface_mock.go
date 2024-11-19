// Code generated by mockery v2.45.1. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	rest "k8s.io/client-go/rest"

	v1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

// MetricsV1beta1InterfaceMock is an autogenerated mock type for the MetricsV1beta1Interface type
type MetricsV1beta1InterfaceMock struct {
	mock.Mock
}

type MetricsV1beta1InterfaceMock_Expecter struct {
	mock *mock.Mock
}

func (_m *MetricsV1beta1InterfaceMock) EXPECT() *MetricsV1beta1InterfaceMock_Expecter {
	return &MetricsV1beta1InterfaceMock_Expecter{mock: &_m.Mock}
}

// NodeMetricses provides a mock function with given fields:
func (_m *MetricsV1beta1InterfaceMock) NodeMetricses() v1beta1.NodeMetricsInterface {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for NodeMetricses")
	}

	var r0 v1beta1.NodeMetricsInterface
	if rf, ok := ret.Get(0).(func() v1beta1.NodeMetricsInterface); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(v1beta1.NodeMetricsInterface)
		}
	}

	return r0
}

// MetricsV1beta1InterfaceMock_NodeMetricses_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NodeMetricses'
type MetricsV1beta1InterfaceMock_NodeMetricses_Call struct {
	*mock.Call
}

// NodeMetricses is a helper method to define mock.On call
func (_e *MetricsV1beta1InterfaceMock_Expecter) NodeMetricses() *MetricsV1beta1InterfaceMock_NodeMetricses_Call {
	return &MetricsV1beta1InterfaceMock_NodeMetricses_Call{Call: _e.mock.On("NodeMetricses")}
}

func (_c *MetricsV1beta1InterfaceMock_NodeMetricses_Call) Run(run func()) *MetricsV1beta1InterfaceMock_NodeMetricses_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MetricsV1beta1InterfaceMock_NodeMetricses_Call) Return(_a0 v1beta1.NodeMetricsInterface) *MetricsV1beta1InterfaceMock_NodeMetricses_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MetricsV1beta1InterfaceMock_NodeMetricses_Call) RunAndReturn(run func() v1beta1.NodeMetricsInterface) *MetricsV1beta1InterfaceMock_NodeMetricses_Call {
	_c.Call.Return(run)
	return _c
}

// PodMetricses provides a mock function with given fields: namespace
func (_m *MetricsV1beta1InterfaceMock) PodMetricses(namespace string) v1beta1.PodMetricsInterface {
	ret := _m.Called(namespace)

	if len(ret) == 0 {
		panic("no return value specified for PodMetricses")
	}

	var r0 v1beta1.PodMetricsInterface
	if rf, ok := ret.Get(0).(func(string) v1beta1.PodMetricsInterface); ok {
		r0 = rf(namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(v1beta1.PodMetricsInterface)
		}
	}

	return r0
}

// MetricsV1beta1InterfaceMock_PodMetricses_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PodMetricses'
type MetricsV1beta1InterfaceMock_PodMetricses_Call struct {
	*mock.Call
}

// PodMetricses is a helper method to define mock.On call
//   - namespace string
func (_e *MetricsV1beta1InterfaceMock_Expecter) PodMetricses(namespace interface{}) *MetricsV1beta1InterfaceMock_PodMetricses_Call {
	return &MetricsV1beta1InterfaceMock_PodMetricses_Call{Call: _e.mock.On("PodMetricses", namespace)}
}

func (_c *MetricsV1beta1InterfaceMock_PodMetricses_Call) Run(run func(namespace string)) *MetricsV1beta1InterfaceMock_PodMetricses_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MetricsV1beta1InterfaceMock_PodMetricses_Call) Return(_a0 v1beta1.PodMetricsInterface) *MetricsV1beta1InterfaceMock_PodMetricses_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MetricsV1beta1InterfaceMock_PodMetricses_Call) RunAndReturn(run func(string) v1beta1.PodMetricsInterface) *MetricsV1beta1InterfaceMock_PodMetricses_Call {
	_c.Call.Return(run)
	return _c
}

// RESTClient provides a mock function with given fields:
func (_m *MetricsV1beta1InterfaceMock) RESTClient() rest.Interface {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RESTClient")
	}

	var r0 rest.Interface
	if rf, ok := ret.Get(0).(func() rest.Interface); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(rest.Interface)
		}
	}

	return r0
}

// MetricsV1beta1InterfaceMock_RESTClient_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RESTClient'
type MetricsV1beta1InterfaceMock_RESTClient_Call struct {
	*mock.Call
}

// RESTClient is a helper method to define mock.On call
func (_e *MetricsV1beta1InterfaceMock_Expecter) RESTClient() *MetricsV1beta1InterfaceMock_RESTClient_Call {
	return &MetricsV1beta1InterfaceMock_RESTClient_Call{Call: _e.mock.On("RESTClient")}
}

func (_c *MetricsV1beta1InterfaceMock_RESTClient_Call) Run(run func()) *MetricsV1beta1InterfaceMock_RESTClient_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MetricsV1beta1InterfaceMock_RESTClient_Call) Return(_a0 rest.Interface) *MetricsV1beta1InterfaceMock_RESTClient_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MetricsV1beta1InterfaceMock_RESTClient_Call) RunAndReturn(run func() rest.Interface) *MetricsV1beta1InterfaceMock_RESTClient_Call {
	_c.Call.Return(run)
	return _c
}

// NewMetricsV1beta1InterfaceMock creates a new instance of MetricsV1beta1InterfaceMock. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMetricsV1beta1InterfaceMock(t interface {
	mock.TestingT
	Cleanup(func())
}) *MetricsV1beta1InterfaceMock {
	mock := &MetricsV1beta1InterfaceMock{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}