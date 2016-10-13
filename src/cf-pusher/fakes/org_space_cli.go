// This file was generated by counterfeiter
package fakes

import "sync"

type OrgSpaceCli struct {
	CreateOrgStub        func(name string) error
	createOrgMutex       sync.RWMutex
	createOrgArgsForCall []struct {
		name string
	}
	createOrgReturns struct {
		result1 error
	}
	CreateSpaceStub        func(name string) error
	createSpaceMutex       sync.RWMutex
	createSpaceArgsForCall []struct {
		name string
	}
	createSpaceReturns struct {
		result1 error
	}
	TargetOrgStub        func(name string) error
	targetOrgMutex       sync.RWMutex
	targetOrgArgsForCall []struct {
		name string
	}
	targetOrgReturns struct {
		result1 error
	}
	TargetSpaceStub        func(name string) error
	targetSpaceMutex       sync.RWMutex
	targetSpaceArgsForCall []struct {
		name string
	}
	targetSpaceReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *OrgSpaceCli) CreateOrg(name string) error {
	fake.createOrgMutex.Lock()
	fake.createOrgArgsForCall = append(fake.createOrgArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("CreateOrg", []interface{}{name})
	fake.createOrgMutex.Unlock()
	if fake.CreateOrgStub != nil {
		return fake.CreateOrgStub(name)
	} else {
		return fake.createOrgReturns.result1
	}
}

func (fake *OrgSpaceCli) CreateOrgCallCount() int {
	fake.createOrgMutex.RLock()
	defer fake.createOrgMutex.RUnlock()
	return len(fake.createOrgArgsForCall)
}

func (fake *OrgSpaceCli) CreateOrgArgsForCall(i int) string {
	fake.createOrgMutex.RLock()
	defer fake.createOrgMutex.RUnlock()
	return fake.createOrgArgsForCall[i].name
}

func (fake *OrgSpaceCli) CreateOrgReturns(result1 error) {
	fake.CreateOrgStub = nil
	fake.createOrgReturns = struct {
		result1 error
	}{result1}
}

func (fake *OrgSpaceCli) CreateSpace(name string) error {
	fake.createSpaceMutex.Lock()
	fake.createSpaceArgsForCall = append(fake.createSpaceArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("CreateSpace", []interface{}{name})
	fake.createSpaceMutex.Unlock()
	if fake.CreateSpaceStub != nil {
		return fake.CreateSpaceStub(name)
	} else {
		return fake.createSpaceReturns.result1
	}
}

func (fake *OrgSpaceCli) CreateSpaceCallCount() int {
	fake.createSpaceMutex.RLock()
	defer fake.createSpaceMutex.RUnlock()
	return len(fake.createSpaceArgsForCall)
}

func (fake *OrgSpaceCli) CreateSpaceArgsForCall(i int) string {
	fake.createSpaceMutex.RLock()
	defer fake.createSpaceMutex.RUnlock()
	return fake.createSpaceArgsForCall[i].name
}

func (fake *OrgSpaceCli) CreateSpaceReturns(result1 error) {
	fake.CreateSpaceStub = nil
	fake.createSpaceReturns = struct {
		result1 error
	}{result1}
}

func (fake *OrgSpaceCli) TargetOrg(name string) error {
	fake.targetOrgMutex.Lock()
	fake.targetOrgArgsForCall = append(fake.targetOrgArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("TargetOrg", []interface{}{name})
	fake.targetOrgMutex.Unlock()
	if fake.TargetOrgStub != nil {
		return fake.TargetOrgStub(name)
	} else {
		return fake.targetOrgReturns.result1
	}
}

func (fake *OrgSpaceCli) TargetOrgCallCount() int {
	fake.targetOrgMutex.RLock()
	defer fake.targetOrgMutex.RUnlock()
	return len(fake.targetOrgArgsForCall)
}

func (fake *OrgSpaceCli) TargetOrgArgsForCall(i int) string {
	fake.targetOrgMutex.RLock()
	defer fake.targetOrgMutex.RUnlock()
	return fake.targetOrgArgsForCall[i].name
}

func (fake *OrgSpaceCli) TargetOrgReturns(result1 error) {
	fake.TargetOrgStub = nil
	fake.targetOrgReturns = struct {
		result1 error
	}{result1}
}

func (fake *OrgSpaceCli) TargetSpace(name string) error {
	fake.targetSpaceMutex.Lock()
	fake.targetSpaceArgsForCall = append(fake.targetSpaceArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("TargetSpace", []interface{}{name})
	fake.targetSpaceMutex.Unlock()
	if fake.TargetSpaceStub != nil {
		return fake.TargetSpaceStub(name)
	} else {
		return fake.targetSpaceReturns.result1
	}
}

func (fake *OrgSpaceCli) TargetSpaceCallCount() int {
	fake.targetSpaceMutex.RLock()
	defer fake.targetSpaceMutex.RUnlock()
	return len(fake.targetSpaceArgsForCall)
}

func (fake *OrgSpaceCli) TargetSpaceArgsForCall(i int) string {
	fake.targetSpaceMutex.RLock()
	defer fake.targetSpaceMutex.RUnlock()
	return fake.targetSpaceArgsForCall[i].name
}

func (fake *OrgSpaceCli) TargetSpaceReturns(result1 error) {
	fake.TargetSpaceStub = nil
	fake.targetSpaceReturns = struct {
		result1 error
	}{result1}
}

func (fake *OrgSpaceCli) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createOrgMutex.RLock()
	defer fake.createOrgMutex.RUnlock()
	fake.createSpaceMutex.RLock()
	defer fake.createSpaceMutex.RUnlock()
	fake.targetOrgMutex.RLock()
	defer fake.targetOrgMutex.RUnlock()
	fake.targetSpaceMutex.RLock()
	defer fake.targetSpaceMutex.RUnlock()
	return fake.invocations
}

func (fake *OrgSpaceCli) recordInvocation(key string, args []interface{}) {
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
