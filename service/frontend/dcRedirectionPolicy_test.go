// Copyright (c) 2017 Uber Technologies, Inc.
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

package frontend

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service/dynamicconfig"
)

type (
	noopDCRedirectionPolicySuite struct {
		suite.Suite
		currentClusterName string
		policy             *NoopRedirectionPolicy
	}

	selectedAPIsForwardingRedirectionPolicySuite struct {
		suite.Suite
		domainName             string
		domainID               string
		currentClusterName     string
		alternativeClusterName string
		mockConfig             *Config
		mockMetadataMgr        *mocks.MetadataManager
		mockClusterMetadata    *mocks.ClusterMetadata
		policy                 *SelectedAPIsForwardingRedirectionPolicy
	}
)

func TestNoopDCRedirectionPolicySuite(t *testing.T) {
	s := new(noopDCRedirectionPolicySuite)
	suite.Run(t, s)
}

func (s *noopDCRedirectionPolicySuite) SetupSuite() {
}

func (s *noopDCRedirectionPolicySuite) TearDownSuite() {

}

func (s *noopDCRedirectionPolicySuite) SetupTest() {
	s.currentClusterName = cluster.TestCurrentClusterName
	s.policy = NewNoopRedirectionPolicy(s.currentClusterName)
}

func (s *noopDCRedirectionPolicySuite) TearDownTest() {

}

func (s *noopDCRedirectionPolicySuite) TestWithDomainRedirect() {
	domainName := "some random domain name"
	domainID := "some random domain ID"
	apiName := "any random API name"
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	err := s.policy.WithDomainIDRedirect(domainID, apiName, callFn)
	s.Nil(err)

	err = s.policy.WithDomainNameRedirect(domainName, apiName, callFn)
	s.Nil(err)

	s.Equal(2, callCount)
}

func TestSelectedAPIsForwardingRedirectionPolicySuite(t *testing.T) {
	s := new(selectedAPIsForwardingRedirectionPolicySuite)
	suite.Run(t, s)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) SetupSuite() {
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TearDownSuite() {

}

func (s *selectedAPIsForwardingRedirectionPolicySuite) SetupTest() {
	s.domainName = "some random domain name"
	s.domainID = "some random domain ID"
	s.currentClusterName = cluster.TestCurrentClusterName
	s.alternativeClusterName = cluster.TestAlternativeClusterName

	logger, err := loggerimpl.NewDevelopment()
	s.Nil(err)

	s.mockConfig = NewConfig(dynamicconfig.NewCollection(dynamicconfig.NewNopClient(), logger), 0, false)
	s.mockMetadataMgr = &mocks.MetadataManager{}
	s.mockClusterMetadata = &mocks.ClusterMetadata{}
	s.mockClusterMetadata.On("IsGlobalDomainEnabled").Return(true)
	domainCache := cache.NewDomainCache(
		s.mockMetadataMgr,
		s.mockClusterMetadata,
		metrics.NewClient(tally.NoopScope, metrics.Frontend),
		logger,
	)
	s.policy = NewSelectedAPIsForwardingPolicy(
		s.currentClusterName,
		s.mockConfig,
		domainCache,
	)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TearDownTest() {

}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_LocalDomain() {
	s.setupLocalDomain()

	apiName := "any random API name"
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
	s.Nil(err)

	err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
	s.Nil(err)

	s.Equal(2, callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_GlobalDomain_OneReplicationCluster() {
	s.setupGlobalDomainWithOneReplicationCluster()

	apiName := "any random API name"
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
	s.Nil(err)

	err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
	s.Nil(err)

	s.Equal(2, callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_GlobalDomain_NoForwarding_DomainNotWhiltelisted() {
	s.setupGlobalDomainWithTwoReplicationCluster(false, true)

	apiName := "any random API name"
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
	s.Nil(err)

	err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
	s.Nil(err)

	s.Equal(2, callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_GlobalDomain_NoForwarding_APINotWhiltelisted() {
	s.setupGlobalDomainWithTwoReplicationCluster(true, true)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs {
		err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
		s.Nil(err)

		err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_CurrentCluster() {
	s.setupGlobalDomainWithTwoReplicationCluster(true, true)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs {
		err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
		s.Nil(err)

		err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_AlternativeCluster() {
	s.setupGlobalDomainWithTwoReplicationCluster(true, false)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.alternativeClusterName, targetCluster)
		return nil
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs {
		err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
		s.Nil(err)

		err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_CurrentClusterToAlternativeCluster() {
	s.setupGlobalDomainWithTwoReplicationCluster(true, true)

	currentClustercallCount := 0
	alternativeClustercallCount := 0
	callFn := func(targetCluster string) error {
		switch targetCluster {
		case s.currentClusterName:
			currentClustercallCount++
			return &shared.DomainNotActiveError{
				CurrentCluster: s.currentClusterName,
				ActiveCluster:  s.alternativeClusterName,
			}
		case s.alternativeClusterName:
			alternativeClustercallCount++
			return nil
		default:
			panic(fmt.Sprintf("unknown cluster name %v", targetCluster))
		}
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs {
		err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
		s.Nil(err)

		err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), currentClustercallCount)
	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), alternativeClustercallCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_AlternativeClusterToCurrentCluster() {
	s.setupGlobalDomainWithTwoReplicationCluster(true, false)

	currentClustercallCount := 0
	alternativeClustercallCount := 0
	callFn := func(targetCluster string) error {
		switch targetCluster {
		case s.currentClusterName:
			currentClustercallCount++
			return nil
		case s.alternativeClusterName:
			alternativeClustercallCount++
			return &shared.DomainNotActiveError{
				CurrentCluster: s.alternativeClusterName,
				ActiveCluster:  s.currentClusterName,
			}
		default:
			panic(fmt.Sprintf("unknown cluster name %v", targetCluster))
		}
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs {
		err := s.policy.WithDomainIDRedirect(s.domainID, apiName, callFn)
		s.Nil(err)

		err = s.policy.WithDomainNameRedirect(s.domainName, apiName, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), currentClustercallCount)
	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyWhitelistedAPIs), alternativeClustercallCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) setupLocalDomain() {
	domainRecord := &persistence.GetDomainResponse{
		Info:   &persistence.DomainInfo{ID: s.domainID, Name: s.domainName},
		Config: &persistence.DomainConfig{},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
		IsGlobalDomain: false,
		TableVersion:   persistence.DomainTableVersionV1,
	}

	s.mockMetadataMgr.On("GetDomain", &persistence.GetDomainRequest{ID: s.domainID}).Return(domainRecord, nil)
	s.mockMetadataMgr.On("GetDomain", &persistence.GetDomainRequest{Name: s.domainName}).Return(domainRecord, nil)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) setupGlobalDomainWithOneReplicationCluster() {
	domainRecord := &persistence.GetDomainResponse{
		Info:   &persistence.DomainInfo{ID: s.domainID, Name: s.domainName},
		Config: &persistence.DomainConfig{},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		IsGlobalDomain: true,
		TableVersion:   persistence.DomainTableVersionV1,
	}

	s.mockMetadataMgr.On("GetDomain", &persistence.GetDomainRequest{ID: s.domainID}).Return(domainRecord, nil)
	s.mockMetadataMgr.On("GetDomain", &persistence.GetDomainRequest{Name: s.domainName}).Return(domainRecord, nil)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) setupGlobalDomainWithTwoReplicationCluster(forwardingEnabled bool, isRecordActive bool) {
	activeCluster := s.alternativeClusterName
	if isRecordActive {
		activeCluster = s.currentClusterName
	}
	domainRecord := &persistence.GetDomainResponse{
		Info:   &persistence.DomainInfo{ID: s.domainID, Name: s.domainName},
		Config: &persistence.DomainConfig{},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: activeCluster,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		IsGlobalDomain: true,
		TableVersion:   persistence.DomainTableVersionV1,
	}

	s.mockMetadataMgr.On("GetDomain", &persistence.GetDomainRequest{ID: s.domainID}).Return(domainRecord, nil)
	s.mockMetadataMgr.On("GetDomain", &persistence.GetDomainRequest{Name: s.domainName}).Return(domainRecord, nil)
	s.mockConfig.EnableDomainNotActiveAutoForwarding = dynamicconfig.GetBoolPropertyFnFilteredByDomain(forwardingEnabled)
}
