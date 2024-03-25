// Code generated by mockery v2.39.0. DO NOT EDIT.

package mocks

import (
	gqlclient "github.com/pluralsh/console-client-go"
	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
)

// ClientMock is an autogenerated mock type for the Client type
type ClientMock struct {
	mock.Mock
}

type ClientMock_Expecter struct {
	mock *mock.Mock
}

func (_m *ClientMock) EXPECT() *ClientMock_Expecter {
	return &ClientMock_Expecter{mock: &_m.Mock}
}

// AddServiceErrors provides a mock function with given fields: id, errs
func (_m *ClientMock) AddServiceErrors(id string, errs []*gqlclient.ServiceErrorAttributes) error {
	ret := _m.Called(id, errs)

	if len(ret) == 0 {
		panic("no return value specified for AddServiceErrors")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []*gqlclient.ServiceErrorAttributes) error); ok {
		r0 = rf(id, errs)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ClientMock_AddServiceErrors_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AddServiceErrors'
type ClientMock_AddServiceErrors_Call struct {
	*mock.Call
}

// AddServiceErrors is a helper method to define mock.On call
//   - id string
//   - errs []*gqlclient.ServiceErrorAttributes
func (_e *ClientMock_Expecter) AddServiceErrors(id interface{}, errs interface{}) *ClientMock_AddServiceErrors_Call {
	return &ClientMock_AddServiceErrors_Call{Call: _e.mock.On("AddServiceErrors", id, errs)}
}

func (_c *ClientMock_AddServiceErrors_Call) Run(run func(id string, errs []*gqlclient.ServiceErrorAttributes)) *ClientMock_AddServiceErrors_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].([]*gqlclient.ServiceErrorAttributes))
	})
	return _c
}

func (_c *ClientMock_AddServiceErrors_Call) Return(_a0 error) *ClientMock_AddServiceErrors_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_AddServiceErrors_Call) RunAndReturn(run func(string, []*gqlclient.ServiceErrorAttributes) error) *ClientMock_AddServiceErrors_Call {
	_c.Call.Return(run)
	return _c
}

// GateExists provides a mock function with given fields: id
func (_m *ClientMock) GateExists(id string) bool {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for GateExists")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(id)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// ClientMock_GateExists_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GateExists'
type ClientMock_GateExists_Call struct {
	*mock.Call
}

// GateExists is a helper method to define mock.On call
//   - id string
func (_e *ClientMock_Expecter) GateExists(id interface{}) *ClientMock_GateExists_Call {
	return &ClientMock_GateExists_Call{Call: _e.mock.On("GateExists", id)}
}

func (_c *ClientMock_GateExists_Call) Run(run func(id string)) *ClientMock_GateExists_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *ClientMock_GateExists_Call) Return(_a0 bool) *ClientMock_GateExists_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_GateExists_Call) RunAndReturn(run func(string) bool) *ClientMock_GateExists_Call {
	_c.Call.Return(run)
	return _c
}

// GetClusterBackup provides a mock function with given fields: clusterID, namespace, name
func (_m *ClientMock) GetClusterBackup(clusterID string, namespace string, name string) (*gqlclient.ClusterBackupFragment, error) {
	ret := _m.Called(clusterID, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterBackup")
	}

	var r0 *gqlclient.ClusterBackupFragment
	var r1 error
	if rf, ok := ret.Get(0).(func(string, string, string) (*gqlclient.ClusterBackupFragment, error)); ok {
		return rf(clusterID, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(string, string, string) *gqlclient.ClusterBackupFragment); ok {
		r0 = rf(clusterID, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.ClusterBackupFragment)
		}
	}

	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(clusterID, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_GetClusterBackup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetClusterBackup'
type ClientMock_GetClusterBackup_Call struct {
	*mock.Call
}

// GetClusterBackup is a helper method to define mock.On call
//   - clusterID string
//   - namespace string
//   - name string
func (_e *ClientMock_Expecter) GetClusterBackup(clusterID interface{}, namespace interface{}, name interface{}) *ClientMock_GetClusterBackup_Call {
	return &ClientMock_GetClusterBackup_Call{Call: _e.mock.On("GetClusterBackup", clusterID, namespace, name)}
}

func (_c *ClientMock_GetClusterBackup_Call) Run(run func(clusterID string, namespace string, name string)) *ClientMock_GetClusterBackup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *ClientMock_GetClusterBackup_Call) Return(_a0 *gqlclient.ClusterBackupFragment, _a1 error) *ClientMock_GetClusterBackup_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_GetClusterBackup_Call) RunAndReturn(run func(string, string, string) (*gqlclient.ClusterBackupFragment, error)) *ClientMock_GetClusterBackup_Call {
	_c.Call.Return(run)
	return _c
}

// GetClusterGate provides a mock function with given fields: id
func (_m *ClientMock) GetClusterGate(id string) (*gqlclient.PipelineGateFragment, error) {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterGate")
	}

	var r0 *gqlclient.PipelineGateFragment
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (*gqlclient.PipelineGateFragment, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(string) *gqlclient.PipelineGateFragment); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.PipelineGateFragment)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_GetClusterGate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetClusterGate'
type ClientMock_GetClusterGate_Call struct {
	*mock.Call
}

// GetClusterGate is a helper method to define mock.On call
//   - id string
func (_e *ClientMock_Expecter) GetClusterGate(id interface{}) *ClientMock_GetClusterGate_Call {
	return &ClientMock_GetClusterGate_Call{Call: _e.mock.On("GetClusterGate", id)}
}

func (_c *ClientMock_GetClusterGate_Call) Run(run func(id string)) *ClientMock_GetClusterGate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *ClientMock_GetClusterGate_Call) Return(_a0 *gqlclient.PipelineGateFragment, _a1 error) *ClientMock_GetClusterGate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_GetClusterGate_Call) RunAndReturn(run func(string) (*gqlclient.PipelineGateFragment, error)) *ClientMock_GetClusterGate_Call {
	_c.Call.Return(run)
	return _c
}

// GetClusterGates provides a mock function with given fields: after, first
func (_m *ClientMock) GetClusterGates(after *string, first *int64) (*gqlclient.PagedClusterGates, error) {
	ret := _m.Called(after, first)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterGates")
	}

	var r0 *gqlclient.PagedClusterGates
	var r1 error
	if rf, ok := ret.Get(0).(func(*string, *int64) (*gqlclient.PagedClusterGates, error)); ok {
		return rf(after, first)
	}
	if rf, ok := ret.Get(0).(func(*string, *int64) *gqlclient.PagedClusterGates); ok {
		r0 = rf(after, first)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.PagedClusterGates)
		}
	}

	if rf, ok := ret.Get(1).(func(*string, *int64) error); ok {
		r1 = rf(after, first)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_GetClusterGates_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetClusterGates'
type ClientMock_GetClusterGates_Call struct {
	*mock.Call
}

// GetClusterGates is a helper method to define mock.On call
//   - after *string
//   - first *int64
func (_e *ClientMock_Expecter) GetClusterGates(after interface{}, first interface{}) *ClientMock_GetClusterGates_Call {
	return &ClientMock_GetClusterGates_Call{Call: _e.mock.On("GetClusterGates", after, first)}
}

func (_c *ClientMock_GetClusterGates_Call) Run(run func(after *string, first *int64)) *ClientMock_GetClusterGates_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*string), args[1].(*int64))
	})
	return _c
}

func (_c *ClientMock_GetClusterGates_Call) Return(_a0 *gqlclient.PagedClusterGates, _a1 error) *ClientMock_GetClusterGates_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_GetClusterGates_Call) RunAndReturn(run func(*string, *int64) (*gqlclient.PagedClusterGates, error)) *ClientMock_GetClusterGates_Call {
	_c.Call.Return(run)
	return _c
}

// GetClusterRestore provides a mock function with given fields: id
func (_m *ClientMock) GetClusterRestore(id string) (*gqlclient.ClusterRestoreFragment, error) {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterRestore")
	}

	var r0 *gqlclient.ClusterRestoreFragment
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (*gqlclient.ClusterRestoreFragment, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(string) *gqlclient.ClusterRestoreFragment); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.ClusterRestoreFragment)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_GetClusterRestore_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetClusterRestore'
type ClientMock_GetClusterRestore_Call struct {
	*mock.Call
}

// GetClusterRestore is a helper method to define mock.On call
//   - id string
func (_e *ClientMock_Expecter) GetClusterRestore(id interface{}) *ClientMock_GetClusterRestore_Call {
	return &ClientMock_GetClusterRestore_Call{Call: _e.mock.On("GetClusterRestore", id)}
}

func (_c *ClientMock_GetClusterRestore_Call) Run(run func(id string)) *ClientMock_GetClusterRestore_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *ClientMock_GetClusterRestore_Call) Return(_a0 *gqlclient.ClusterRestoreFragment, _a1 error) *ClientMock_GetClusterRestore_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_GetClusterRestore_Call) RunAndReturn(run func(string) (*gqlclient.ClusterRestoreFragment, error)) *ClientMock_GetClusterRestore_Call {
	_c.Call.Return(run)
	return _c
}

// GetCredentials provides a mock function with given fields:
func (_m *ClientMock) GetCredentials() (string, string) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetCredentials")
	}

	var r0 string
	var r1 string
	if rf, ok := ret.Get(0).(func() (string, string)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() string); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(string)
	}

	return r0, r1
}

// ClientMock_GetCredentials_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCredentials'
type ClientMock_GetCredentials_Call struct {
	*mock.Call
}

// GetCredentials is a helper method to define mock.On call
func (_e *ClientMock_Expecter) GetCredentials() *ClientMock_GetCredentials_Call {
	return &ClientMock_GetCredentials_Call{Call: _e.mock.On("GetCredentials")}
}

func (_c *ClientMock_GetCredentials_Call) Run(run func()) *ClientMock_GetCredentials_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ClientMock_GetCredentials_Call) Return(url string, token string) *ClientMock_GetCredentials_Call {
	_c.Call.Return(url, token)
	return _c
}

func (_c *ClientMock_GetCredentials_Call) RunAndReturn(run func() (string, string)) *ClientMock_GetCredentials_Call {
	_c.Call.Return(run)
	return _c
}

// GetService provides a mock function with given fields: id
func (_m *ClientMock) GetService(id string) (*gqlclient.GetServiceDeploymentForAgent_ServiceDeployment, error) {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for GetService")
	}

	var r0 *gqlclient.GetServiceDeploymentForAgent_ServiceDeployment
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (*gqlclient.GetServiceDeploymentForAgent_ServiceDeployment, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(string) *gqlclient.GetServiceDeploymentForAgent_ServiceDeployment); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.GetServiceDeploymentForAgent_ServiceDeployment)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_GetService_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetService'
type ClientMock_GetService_Call struct {
	*mock.Call
}

// GetService is a helper method to define mock.On call
//   - id string
func (_e *ClientMock_Expecter) GetService(id interface{}) *ClientMock_GetService_Call {
	return &ClientMock_GetService_Call{Call: _e.mock.On("GetService", id)}
}

func (_c *ClientMock_GetService_Call) Run(run func(id string)) *ClientMock_GetService_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *ClientMock_GetService_Call) Return(_a0 *gqlclient.GetServiceDeploymentForAgent_ServiceDeployment, _a1 error) *ClientMock_GetService_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_GetService_Call) RunAndReturn(run func(string) (*gqlclient.GetServiceDeploymentForAgent_ServiceDeployment, error)) *ClientMock_GetService_Call {
	_c.Call.Return(run)
	return _c
}

// GetServices provides a mock function with given fields: after, first
func (_m *ClientMock) GetServices(after *string, first *int64) (*gqlclient.PagedClusterServices, error) {
	ret := _m.Called(after, first)

	if len(ret) == 0 {
		panic("no return value specified for GetServices")
	}

	var r0 *gqlclient.PagedClusterServices
	var r1 error
	if rf, ok := ret.Get(0).(func(*string, *int64) (*gqlclient.PagedClusterServices, error)); ok {
		return rf(after, first)
	}
	if rf, ok := ret.Get(0).(func(*string, *int64) *gqlclient.PagedClusterServices); ok {
		r0 = rf(after, first)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.PagedClusterServices)
		}
	}

	if rf, ok := ret.Get(1).(func(*string, *int64) error); ok {
		r1 = rf(after, first)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_GetServices_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetServices'
type ClientMock_GetServices_Call struct {
	*mock.Call
}

// GetServices is a helper method to define mock.On call
//   - after *string
//   - first *int64
func (_e *ClientMock_Expecter) GetServices(after interface{}, first interface{}) *ClientMock_GetServices_Call {
	return &ClientMock_GetServices_Call{Call: _e.mock.On("GetServices", after, first)}
}

func (_c *ClientMock_GetServices_Call) Run(run func(after *string, first *int64)) *ClientMock_GetServices_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*string), args[1].(*int64))
	})
	return _c
}

func (_c *ClientMock_GetServices_Call) Return(_a0 *gqlclient.PagedClusterServices, _a1 error) *ClientMock_GetServices_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_GetServices_Call) RunAndReturn(run func(*string, *int64) (*gqlclient.PagedClusterServices, error)) *ClientMock_GetServices_Call {
	_c.Call.Return(run)
	return _c
}

// MyCluster provides a mock function with given fields:
func (_m *ClientMock) MyCluster() (*gqlclient.MyCluster, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for MyCluster")
	}

	var r0 *gqlclient.MyCluster
	var r1 error
	if rf, ok := ret.Get(0).(func() (*gqlclient.MyCluster, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *gqlclient.MyCluster); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.MyCluster)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_MyCluster_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'MyCluster'
type ClientMock_MyCluster_Call struct {
	*mock.Call
}

// MyCluster is a helper method to define mock.On call
func (_e *ClientMock_Expecter) MyCluster() *ClientMock_MyCluster_Call {
	return &ClientMock_MyCluster_Call{Call: _e.mock.On("MyCluster")}
}

func (_c *ClientMock_MyCluster_Call) Run(run func()) *ClientMock_MyCluster_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ClientMock_MyCluster_Call) Return(_a0 *gqlclient.MyCluster, _a1 error) *ClientMock_MyCluster_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_MyCluster_Call) RunAndReturn(run func() (*gqlclient.MyCluster, error)) *ClientMock_MyCluster_Call {
	_c.Call.Return(run)
	return _c
}

// ParsePipelineGateCR provides a mock function with given fields: pgFragment, operatorNamespace
func (_m *ClientMock) ParsePipelineGateCR(pgFragment *gqlclient.PipelineGateFragment, operatorNamespace string) (*v1alpha1.PipelineGate, error) {
	ret := _m.Called(pgFragment, operatorNamespace)

	if len(ret) == 0 {
		panic("no return value specified for ParsePipelineGateCR")
	}

	var r0 *v1alpha1.PipelineGate
	var r1 error
	if rf, ok := ret.Get(0).(func(*gqlclient.PipelineGateFragment, string) (*v1alpha1.PipelineGate, error)); ok {
		return rf(pgFragment, operatorNamespace)
	}
	if rf, ok := ret.Get(0).(func(*gqlclient.PipelineGateFragment, string) *v1alpha1.PipelineGate); ok {
		r0 = rf(pgFragment, operatorNamespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.PipelineGate)
		}
	}

	if rf, ok := ret.Get(1).(func(*gqlclient.PipelineGateFragment, string) error); ok {
		r1 = rf(pgFragment, operatorNamespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_ParsePipelineGateCR_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ParsePipelineGateCR'
type ClientMock_ParsePipelineGateCR_Call struct {
	*mock.Call
}

// ParsePipelineGateCR is a helper method to define mock.On call
//   - pgFragment *gqlclient.PipelineGateFragment
//   - operatorNamespace string
func (_e *ClientMock_Expecter) ParsePipelineGateCR(pgFragment interface{}, operatorNamespace interface{}) *ClientMock_ParsePipelineGateCR_Call {
	return &ClientMock_ParsePipelineGateCR_Call{Call: _e.mock.On("ParsePipelineGateCR", pgFragment, operatorNamespace)}
}

func (_c *ClientMock_ParsePipelineGateCR_Call) Run(run func(pgFragment *gqlclient.PipelineGateFragment, operatorNamespace string)) *ClientMock_ParsePipelineGateCR_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*gqlclient.PipelineGateFragment), args[1].(string))
	})
	return _c
}

func (_c *ClientMock_ParsePipelineGateCR_Call) Return(_a0 *v1alpha1.PipelineGate, _a1 error) *ClientMock_ParsePipelineGateCR_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_ParsePipelineGateCR_Call) RunAndReturn(run func(*gqlclient.PipelineGateFragment, string) (*v1alpha1.PipelineGate, error)) *ClientMock_ParsePipelineGateCR_Call {
	_c.Call.Return(run)
	return _c
}

// Ping provides a mock function with given fields: vsn
func (_m *ClientMock) Ping(vsn string) error {
	ret := _m.Called(vsn)

	if len(ret) == 0 {
		panic("no return value specified for Ping")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(vsn)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ClientMock_Ping_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Ping'
type ClientMock_Ping_Call struct {
	*mock.Call
}

// Ping is a helper method to define mock.On call
//   - vsn string
func (_e *ClientMock_Expecter) Ping(vsn interface{}) *ClientMock_Ping_Call {
	return &ClientMock_Ping_Call{Call: _e.mock.On("Ping", vsn)}
}

func (_c *ClientMock_Ping_Call) Run(run func(vsn string)) *ClientMock_Ping_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *ClientMock_Ping_Call) Return(_a0 error) *ClientMock_Ping_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_Ping_Call) RunAndReturn(run func(string) error) *ClientMock_Ping_Call {
	_c.Call.Return(run)
	return _c
}

// PingCluster provides a mock function with given fields: attributes
func (_m *ClientMock) PingCluster(attributes gqlclient.ClusterPing) error {
	ret := _m.Called(attributes)

	if len(ret) == 0 {
		panic("no return value specified for PingCluster")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(gqlclient.ClusterPing) error); ok {
		r0 = rf(attributes)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ClientMock_PingCluster_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PingCluster'
type ClientMock_PingCluster_Call struct {
	*mock.Call
}

// PingCluster is a helper method to define mock.On call
//   - attributes gqlclient.ClusterPing
func (_e *ClientMock_Expecter) PingCluster(attributes interface{}) *ClientMock_PingCluster_Call {
	return &ClientMock_PingCluster_Call{Call: _e.mock.On("PingCluster", attributes)}
}

func (_c *ClientMock_PingCluster_Call) Run(run func(attributes gqlclient.ClusterPing)) *ClientMock_PingCluster_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(gqlclient.ClusterPing))
	})
	return _c
}

func (_c *ClientMock_PingCluster_Call) Return(_a0 error) *ClientMock_PingCluster_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_PingCluster_Call) RunAndReturn(run func(gqlclient.ClusterPing) error) *ClientMock_PingCluster_Call {
	_c.Call.Return(run)
	return _c
}

// RegisterRuntimeServices provides a mock function with given fields: svcs, serviceId
func (_m *ClientMock) RegisterRuntimeServices(svcs map[string]string, serviceId *string) error {
	ret := _m.Called(svcs, serviceId)

	if len(ret) == 0 {
		panic("no return value specified for RegisterRuntimeServices")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(map[string]string, *string) error); ok {
		r0 = rf(svcs, serviceId)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ClientMock_RegisterRuntimeServices_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RegisterRuntimeServices'
type ClientMock_RegisterRuntimeServices_Call struct {
	*mock.Call
}

// RegisterRuntimeServices is a helper method to define mock.On call
//   - svcs map[string]string
//   - serviceId *string
func (_e *ClientMock_Expecter) RegisterRuntimeServices(svcs interface{}, serviceId interface{}) *ClientMock_RegisterRuntimeServices_Call {
	return &ClientMock_RegisterRuntimeServices_Call{Call: _e.mock.On("RegisterRuntimeServices", svcs, serviceId)}
}

func (_c *ClientMock_RegisterRuntimeServices_Call) Run(run func(svcs map[string]string, serviceId *string)) *ClientMock_RegisterRuntimeServices_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(map[string]string), args[1].(*string))
	})
	return _c
}

func (_c *ClientMock_RegisterRuntimeServices_Call) Return(_a0 error) *ClientMock_RegisterRuntimeServices_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_RegisterRuntimeServices_Call) RunAndReturn(run func(map[string]string, *string) error) *ClientMock_RegisterRuntimeServices_Call {
	_c.Call.Return(run)
	return _c
}

// SaveClusterBackup provides a mock function with given fields: attrs
func (_m *ClientMock) SaveClusterBackup(attrs gqlclient.BackupAttributes) (*gqlclient.ClusterBackupFragment, error) {
	ret := _m.Called(attrs)

	if len(ret) == 0 {
		panic("no return value specified for SaveClusterBackup")
	}

	var r0 *gqlclient.ClusterBackupFragment
	var r1 error
	if rf, ok := ret.Get(0).(func(gqlclient.BackupAttributes) (*gqlclient.ClusterBackupFragment, error)); ok {
		return rf(attrs)
	}
	if rf, ok := ret.Get(0).(func(gqlclient.BackupAttributes) *gqlclient.ClusterBackupFragment); ok {
		r0 = rf(attrs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.ClusterBackupFragment)
		}
	}

	if rf, ok := ret.Get(1).(func(gqlclient.BackupAttributes) error); ok {
		r1 = rf(attrs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_SaveClusterBackup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SaveClusterBackup'
type ClientMock_SaveClusterBackup_Call struct {
	*mock.Call
}

// SaveClusterBackup is a helper method to define mock.On call
//   - attrs gqlclient.BackupAttributes
func (_e *ClientMock_Expecter) SaveClusterBackup(attrs interface{}) *ClientMock_SaveClusterBackup_Call {
	return &ClientMock_SaveClusterBackup_Call{Call: _e.mock.On("SaveClusterBackup", attrs)}
}

func (_c *ClientMock_SaveClusterBackup_Call) Run(run func(attrs gqlclient.BackupAttributes)) *ClientMock_SaveClusterBackup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(gqlclient.BackupAttributes))
	})
	return _c
}

func (_c *ClientMock_SaveClusterBackup_Call) Return(_a0 *gqlclient.ClusterBackupFragment, _a1 error) *ClientMock_SaveClusterBackup_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_SaveClusterBackup_Call) RunAndReturn(run func(gqlclient.BackupAttributes) (*gqlclient.ClusterBackupFragment, error)) *ClientMock_SaveClusterBackup_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateClusterRestore provides a mock function with given fields: id, attrs
func (_m *ClientMock) UpdateClusterRestore(id string, attrs gqlclient.RestoreAttributes) (*gqlclient.ClusterRestoreFragment, error) {
	ret := _m.Called(id, attrs)

	if len(ret) == 0 {
		panic("no return value specified for UpdateClusterRestore")
	}

	var r0 *gqlclient.ClusterRestoreFragment
	var r1 error
	if rf, ok := ret.Get(0).(func(string, gqlclient.RestoreAttributes) (*gqlclient.ClusterRestoreFragment, error)); ok {
		return rf(id, attrs)
	}
	if rf, ok := ret.Get(0).(func(string, gqlclient.RestoreAttributes) *gqlclient.ClusterRestoreFragment); ok {
		r0 = rf(id, attrs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.ClusterRestoreFragment)
		}
	}

	if rf, ok := ret.Get(1).(func(string, gqlclient.RestoreAttributes) error); ok {
		r1 = rf(id, attrs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_UpdateClusterRestore_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateClusterRestore'
type ClientMock_UpdateClusterRestore_Call struct {
	*mock.Call
}

// UpdateClusterRestore is a helper method to define mock.On call
//   - id string
//   - attrs gqlclient.RestoreAttributes
func (_e *ClientMock_Expecter) UpdateClusterRestore(id interface{}, attrs interface{}) *ClientMock_UpdateClusterRestore_Call {
	return &ClientMock_UpdateClusterRestore_Call{Call: _e.mock.On("UpdateClusterRestore", id, attrs)}
}

func (_c *ClientMock_UpdateClusterRestore_Call) Run(run func(id string, attrs gqlclient.RestoreAttributes)) *ClientMock_UpdateClusterRestore_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(gqlclient.RestoreAttributes))
	})
	return _c
}

func (_c *ClientMock_UpdateClusterRestore_Call) Return(_a0 *gqlclient.ClusterRestoreFragment, _a1 error) *ClientMock_UpdateClusterRestore_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_UpdateClusterRestore_Call) RunAndReturn(run func(string, gqlclient.RestoreAttributes) (*gqlclient.ClusterRestoreFragment, error)) *ClientMock_UpdateClusterRestore_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateComponents provides a mock function with given fields: id, components, errs
func (_m *ClientMock) UpdateComponents(id string, components []*gqlclient.ComponentAttributes, errs []*gqlclient.ServiceErrorAttributes) error {
	ret := _m.Called(id, components, errs)

	if len(ret) == 0 {
		panic("no return value specified for UpdateComponents")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []*gqlclient.ComponentAttributes, []*gqlclient.ServiceErrorAttributes) error); ok {
		r0 = rf(id, components, errs)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ClientMock_UpdateComponents_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateComponents'
type ClientMock_UpdateComponents_Call struct {
	*mock.Call
}

// UpdateComponents is a helper method to define mock.On call
//   - id string
//   - components []*gqlclient.ComponentAttributes
//   - errs []*gqlclient.ServiceErrorAttributes
func (_e *ClientMock_Expecter) UpdateComponents(id interface{}, components interface{}, errs interface{}) *ClientMock_UpdateComponents_Call {
	return &ClientMock_UpdateComponents_Call{Call: _e.mock.On("UpdateComponents", id, components, errs)}
}

func (_c *ClientMock_UpdateComponents_Call) Run(run func(id string, components []*gqlclient.ComponentAttributes, errs []*gqlclient.ServiceErrorAttributes)) *ClientMock_UpdateComponents_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].([]*gqlclient.ComponentAttributes), args[2].([]*gqlclient.ServiceErrorAttributes))
	})
	return _c
}

func (_c *ClientMock_UpdateComponents_Call) Return(_a0 error) *ClientMock_UpdateComponents_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_UpdateComponents_Call) RunAndReturn(run func(string, []*gqlclient.ComponentAttributes, []*gqlclient.ServiceErrorAttributes) error) *ClientMock_UpdateComponents_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateGate provides a mock function with given fields: id, attributes
func (_m *ClientMock) UpdateGate(id string, attributes gqlclient.GateUpdateAttributes) error {
	ret := _m.Called(id, attributes)

	if len(ret) == 0 {
		panic("no return value specified for UpdateGate")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, gqlclient.GateUpdateAttributes) error); ok {
		r0 = rf(id, attributes)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ClientMock_UpdateGate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateGate'
type ClientMock_UpdateGate_Call struct {
	*mock.Call
}

// UpdateGate is a helper method to define mock.On call
//   - id string
//   - attributes gqlclient.GateUpdateAttributes
func (_e *ClientMock_Expecter) UpdateGate(id interface{}, attributes interface{}) *ClientMock_UpdateGate_Call {
	return &ClientMock_UpdateGate_Call{Call: _e.mock.On("UpdateGate", id, attributes)}
}

func (_c *ClientMock_UpdateGate_Call) Run(run func(id string, attributes gqlclient.GateUpdateAttributes)) *ClientMock_UpdateGate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(gqlclient.GateUpdateAttributes))
	})
	return _c
}

func (_c *ClientMock_UpdateGate_Call) Return(_a0 error) *ClientMock_UpdateGate_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ClientMock_UpdateGate_Call) RunAndReturn(run func(string, gqlclient.GateUpdateAttributes) error) *ClientMock_UpdateGate_Call {
	_c.Call.Return(run)
	return _c
}

// UpsertConstraints provides a mock function with given fields: constraints
func (_m *ClientMock) UpsertConstraints(constraints []*gqlclient.PolicyConstraintAttributes) (*gqlclient.UpsertPolicyConstraints, error) {
	ret := _m.Called(constraints)

	if len(ret) == 0 {
		panic("no return value specified for UpsertConstraints")
	}

	var r0 *gqlclient.UpsertPolicyConstraints
	var r1 error
	if rf, ok := ret.Get(0).(func([]*gqlclient.PolicyConstraintAttributes) (*gqlclient.UpsertPolicyConstraints, error)); ok {
		return rf(constraints)
	}
	if rf, ok := ret.Get(0).(func([]*gqlclient.PolicyConstraintAttributes) *gqlclient.UpsertPolicyConstraints); ok {
		r0 = rf(constraints)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gqlclient.UpsertPolicyConstraints)
		}
	}

	if rf, ok := ret.Get(1).(func([]*gqlclient.PolicyConstraintAttributes) error); ok {
		r1 = rf(constraints)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClientMock_UpsertConstraints_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpsertConstraints'
type ClientMock_UpsertConstraints_Call struct {
	*mock.Call
}

// UpsertConstraints is a helper method to define mock.On call
//   - constraints []*gqlclient.PolicyConstraintAttributes
func (_e *ClientMock_Expecter) UpsertConstraints(constraints interface{}) *ClientMock_UpsertConstraints_Call {
	return &ClientMock_UpsertConstraints_Call{Call: _e.mock.On("UpsertConstraints", constraints)}
}

func (_c *ClientMock_UpsertConstraints_Call) Run(run func(constraints []*gqlclient.PolicyConstraintAttributes)) *ClientMock_UpsertConstraints_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]*gqlclient.PolicyConstraintAttributes))
	})
	return _c
}

func (_c *ClientMock_UpsertConstraints_Call) Return(_a0 *gqlclient.UpsertPolicyConstraints, _a1 error) *ClientMock_UpsertConstraints_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ClientMock_UpsertConstraints_Call) RunAndReturn(run func([]*gqlclient.PolicyConstraintAttributes) (*gqlclient.UpsertPolicyConstraints, error)) *ClientMock_UpsertConstraints_Call {
	_c.Call.Return(run)
	return _c
}

// NewClientMock creates a new instance of ClientMock. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewClientMock(t interface {
	mock.TestingT
	Cleanup(func())
}) *ClientMock {
	mock := &ClientMock{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
