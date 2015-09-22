// This file was generated by counterfeiter
package credential_fakes

import (
	"sync"

	"github.com/cloudfoundry-incubator/diego-ssh/cf-plugin/models/credential"
)

type FakeCredentialFactory struct {
	AuthorizationCodeStub        func() (string, error)
	authorizationCodeMutex       sync.RWMutex
	authorizationCodeArgsForCall []struct{}
	authorizationCodeReturns     struct {
		result1 string
		result2 error
	}
}

func (fake *FakeCredentialFactory) AuthorizationCode() (string, error) {
	fake.authorizationCodeMutex.Lock()
	fake.authorizationCodeArgsForCall = append(fake.authorizationCodeArgsForCall, struct{}{})
	fake.authorizationCodeMutex.Unlock()
	if fake.AuthorizationCodeStub != nil {
		return fake.AuthorizationCodeStub()
	} else {
		return fake.authorizationCodeReturns.result1, fake.authorizationCodeReturns.result2
	}
}

func (fake *FakeCredentialFactory) AuthorizationCodeCallCount() int {
	fake.authorizationCodeMutex.RLock()
	defer fake.authorizationCodeMutex.RUnlock()
	return len(fake.authorizationCodeArgsForCall)
}

func (fake *FakeCredentialFactory) AuthorizationCodeReturns(result1 string, result2 error) {
	fake.AuthorizationCodeStub = nil
	fake.authorizationCodeReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

var _ credential.CredentialFactory = new(FakeCredentialFactory)
