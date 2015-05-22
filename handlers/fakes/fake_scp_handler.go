// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/cloudfoundry-incubator/diego-ssh/handlers"
	"golang.org/x/crypto/ssh"
)

type FakeSCPHandler struct {
	HandleSCPRequestStub        func(request *ssh.Request, cmd string) error
	handleSCPRequestMutex       sync.RWMutex
	handleSCPRequestArgsForCall []struct {
		request *ssh.Request
		cmd     string
	}
	handleSCPRequestReturns struct {
		result1 error
	}
}

func (fake *FakeSCPHandler) HandleSCPRequest(request *ssh.Request, cmd string) error {
	fake.handleSCPRequestMutex.Lock()
	fake.handleSCPRequestArgsForCall = append(fake.handleSCPRequestArgsForCall, struct {
		request *ssh.Request
		cmd     string
	}{request, cmd})
	fake.handleSCPRequestMutex.Unlock()
	if fake.HandleSCPRequestStub != nil {
		return fake.HandleSCPRequestStub(request, cmd)
	} else {
		return fake.handleSCPRequestReturns.result1
	}
}

func (fake *FakeSCPHandler) HandleSCPRequestCallCount() int {
	fake.handleSCPRequestMutex.RLock()
	defer fake.handleSCPRequestMutex.RUnlock()
	return len(fake.handleSCPRequestArgsForCall)
}

func (fake *FakeSCPHandler) HandleSCPRequestArgsForCall(i int) (*ssh.Request, string) {
	fake.handleSCPRequestMutex.RLock()
	defer fake.handleSCPRequestMutex.RUnlock()
	return fake.handleSCPRequestArgsForCall[i].request, fake.handleSCPRequestArgsForCall[i].cmd
}

func (fake *FakeSCPHandler) HandleSCPRequestReturns(result1 error) {
	fake.HandleSCPRequestStub = nil
	fake.handleSCPRequestReturns = struct {
		result1 error
	}{result1}
}

var _ handlers.SCPHandler = new(FakeSCPHandler)
