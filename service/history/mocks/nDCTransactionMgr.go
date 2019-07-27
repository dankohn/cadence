// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import context "context"

import mock "github.com/stretchr/testify/mock"
import persistence "github.com/uber/cadence/common/persistence"
import time "time"

// nDCTransactionMgr is an autogenerated mock type for the nDCTransactionMgr type
type nDCTransactionMgr struct {
	mock.Mock
}

// backfillWorkflow provides a mock function with given fields: ctx, now, targetWorkflow, targetWorkflowEvents
func (_m *nDCTransactionMgr) backfillWorkflow(ctx context.Context, now time.Time, targetWorkflow history.nDCWorkflow, targetWorkflowEvents *persistence.WorkflowEvents) error {
	ret := _m.Called(ctx, now, targetWorkflow, targetWorkflowEvents)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, history.nDCWorkflow, *persistence.WorkflowEvents) error); ok {
		r0 = rf(ctx, now, targetWorkflow, targetWorkflowEvents)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// createWorkflow provides a mock function with given fields: ctx, now, targetWorkflow
func (_m *nDCTransactionMgr) createWorkflow(ctx context.Context, now time.Time, targetWorkflow history.nDCWorkflow) error {
	ret := _m.Called(ctx, now, targetWorkflow)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, history.nDCWorkflow) error); ok {
		r0 = rf(ctx, now, targetWorkflow)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// updateWorkflow provides a mock function with given fields: ctx, now, isWorkflowRebuilt, targetWorkflow, newWorkflow
func (_m *nDCTransactionMgr) updateWorkflow(ctx context.Context, now time.Time, isWorkflowRebuilt bool, targetWorkflow history.nDCWorkflow, newWorkflow history.nDCWorkflow) error {
	ret := _m.Called(ctx, now, isWorkflowRebuilt, targetWorkflow, newWorkflow)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, bool, history.nDCWorkflow, history.nDCWorkflow) error); ok {
		r0 = rf(ctx, now, isWorkflowRebuilt, targetWorkflow, newWorkflow)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
