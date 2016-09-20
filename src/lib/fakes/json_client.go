// This file was generated by counterfeiter
package fakes

import "sync"

type JSONClient struct {
	DoStub        func(method, route string, reqData, respData interface{}) error
	doMutex       sync.RWMutex
	doArgsForCall []struct {
		method   string
		route    string
		reqData  interface{}
		respData interface{}
	}
	doReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *JSONClient) Do(method string, route string, reqData interface{}, respData interface{}) error {
	fake.doMutex.Lock()
	fake.doArgsForCall = append(fake.doArgsForCall, struct {
		method   string
		route    string
		reqData  interface{}
		respData interface{}
	}{method, route, reqData, respData})
	fake.recordInvocation("Do", []interface{}{method, route, reqData, respData})
	fake.doMutex.Unlock()
	if fake.DoStub != nil {
		return fake.DoStub(method, route, reqData, respData)
	} else {
		return fake.doReturns.result1
	}
}

func (fake *JSONClient) DoCallCount() int {
	fake.doMutex.RLock()
	defer fake.doMutex.RUnlock()
	return len(fake.doArgsForCall)
}

func (fake *JSONClient) DoArgsForCall(i int) (string, string, interface{}, interface{}) {
	fake.doMutex.RLock()
	defer fake.doMutex.RUnlock()
	return fake.doArgsForCall[i].method, fake.doArgsForCall[i].route, fake.doArgsForCall[i].reqData, fake.doArgsForCall[i].respData
}

func (fake *JSONClient) DoReturns(result1 error) {
	fake.DoStub = nil
	fake.doReturns = struct {
		result1 error
	}{result1}
}

func (fake *JSONClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.doMutex.RLock()
	defer fake.doMutex.RUnlock()
	return fake.invocations
}

func (fake *JSONClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
