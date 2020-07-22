// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator (interfaces: ValuesProvider)

// Package genericactuator is a generated GoMock package.
package genericactuator

import (
	context "context"
	v1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensions "github.com/gardener/gardener/pkg/extensions"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockValuesProvider is a mock of ValuesProvider interface
type MockValuesProvider struct {
	ctrl     *gomock.Controller
	recorder *MockValuesProviderMockRecorder
}

// MockValuesProviderMockRecorder is the mock recorder for MockValuesProvider
type MockValuesProviderMockRecorder struct {
	mock *MockValuesProvider
}

// NewMockValuesProvider creates a new mock instance
func NewMockValuesProvider(ctrl *gomock.Controller) *MockValuesProvider {
	mock := &MockValuesProvider{ctrl: ctrl}
	mock.recorder = &MockValuesProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockValuesProvider) EXPECT() *MockValuesProviderMockRecorder {
	return m.recorder
}

// GetConfigChartValues mocks base method
func (m *MockValuesProvider) GetConfigChartValues(arg0 context.Context, arg1 *v1alpha1.ControlPlane, arg2 *extensions.Cluster) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetConfigChartValues", arg0, arg1, arg2)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConfigChartValues indicates an expected call of GetConfigChartValues
func (mr *MockValuesProviderMockRecorder) GetConfigChartValues(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConfigChartValues", reflect.TypeOf((*MockValuesProvider)(nil).GetConfigChartValues), arg0, arg1, arg2)
}

// GetControlPlaneChartValues mocks base method
func (m *MockValuesProvider) GetControlPlaneChartValues(arg0 context.Context, arg1 *v1alpha1.ControlPlane, arg2 *extensions.Cluster, arg3 map[string]string, arg4 bool) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetControlPlaneChartValues", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetControlPlaneChartValues indicates an expected call of GetControlPlaneChartValues
func (mr *MockValuesProviderMockRecorder) GetControlPlaneChartValues(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetControlPlaneChartValues", reflect.TypeOf((*MockValuesProvider)(nil).GetControlPlaneChartValues), arg0, arg1, arg2, arg3, arg4)
}

// GetControlPlaneExposureChartValues mocks base method
func (m *MockValuesProvider) GetControlPlaneExposureChartValues(arg0 context.Context, arg1 *v1alpha1.ControlPlane, arg2 *extensions.Cluster, arg3 map[string]string) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetControlPlaneExposureChartValues", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetControlPlaneExposureChartValues indicates an expected call of GetControlPlaneExposureChartValues
func (mr *MockValuesProviderMockRecorder) GetControlPlaneExposureChartValues(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetControlPlaneExposureChartValues", reflect.TypeOf((*MockValuesProvider)(nil).GetControlPlaneExposureChartValues), arg0, arg1, arg2, arg3)
}

// GetControlPlaneShootChartValues mocks base method
func (m *MockValuesProvider) GetControlPlaneShootChartValues(arg0 context.Context, arg1 *v1alpha1.ControlPlane, arg2 *extensions.Cluster, arg3 map[string]string) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetControlPlaneShootChartValues", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetControlPlaneShootChartValues indicates an expected call of GetControlPlaneShootChartValues
func (mr *MockValuesProviderMockRecorder) GetControlPlaneShootChartValues(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetControlPlaneShootChartValues", reflect.TypeOf((*MockValuesProvider)(nil).GetControlPlaneShootChartValues), arg0, arg1, arg2, arg3)
}

// GetStorageClassesChartValues mocks base method
func (m *MockValuesProvider) GetStorageClassesChartValues(arg0 context.Context, arg1 *v1alpha1.ControlPlane, arg2 *extensions.Cluster) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageClassesChartValues", arg0, arg1, arg2)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStorageClassesChartValues indicates an expected call of GetStorageClassesChartValues
func (mr *MockValuesProviderMockRecorder) GetStorageClassesChartValues(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageClassesChartValues", reflect.TypeOf((*MockValuesProvider)(nil).GetStorageClassesChartValues), arg0, arg1, arg2)
}
