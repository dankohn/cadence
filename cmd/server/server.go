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

package main

import (
	"log"
	"time"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/service/config"
	"github.com/uber/cadence/common/service/dynamicconfig"
	"github.com/uber/cadence/service/frontend"
	"github.com/uber/cadence/service/history"
	"github.com/uber/cadence/service/matching"
	"github.com/uber/cadence/service/worker"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/zap"
)

type (
	server struct {
		name   string
		cfg    *config.Config
		doneC  chan struct{}
		daemon common.Daemon
	}
)

const (
	frontendService = "frontend"
	historyService  = "history"
	matchingService = "matching"
	workerService   = "worker"
)

// newServer returns a new instance of a daemon
// that represents a cadence service
func newServer(service string, cfg *config.Config) common.Daemon {
	return &server{
		cfg:   cfg,
		name:  service,
		doneC: make(chan struct{}),
	}
}

// Start starts the server
func (s *server) Start() {
	if _, ok := s.cfg.Services[s.name]; !ok {
		log.Fatalf("`%v` service missing config", s)
	}
	s.daemon = s.startService()
}

// Stop stops the server
func (s *server) Stop() {

	if s.daemon == nil {
		return
	}

	select {
	case <-s.doneC:
	default:
		s.daemon.Stop()
		select {
		case <-s.doneC:
		case <-time.After(time.Minute):
			log.Printf("timed out waiting for server %v to exit\n", s.name)
		}
	}
}

// startService starts a service with the given name and config
func (s *server) startService() common.Daemon {

	var err error

	params := service.BootstrapParams{}
	params.Name = "cadence-" + s.name
	params.Logger = loggerimpl.NewLogger(s.cfg.Log.NewZapLogger())
	params.PersistenceConfig = s.cfg.Persistence

	params.MembershipFactory, err = s.cfg.Ringpop.NewFactory(params.Logger, params.Name)
	if err != nil {
		log.Fatalf("error creating ringpop factory: %v", err)
	}

	params.DynamicConfig, err = dynamicconfig.NewFileBasedClient(&s.cfg.DynamicConfigClient, params.Logger.WithTags(tag.Service(params.Name)), s.doneC)
	if err != nil {
		log.Printf("error creating file based dynamic config client, use no-op config client instead. error: %v", err)
		params.DynamicConfig = dynamicconfig.NewNopClient()
	}
	dc := dynamicconfig.NewCollection(params.DynamicConfig, params.Logger)

	svcCfg := s.cfg.Services[s.name]
	params.MetricScope = svcCfg.Metrics.NewScope(params.Logger)
	params.RPCFactory = svcCfg.RPC.NewFactory(params.Name, params.Logger)
	params.PProfInitializer = svcCfg.PProf.NewInitializer(params.Logger)

	params.DCRedirectionPolicy = s.cfg.DCRedirectionPolicy

	params.MetricsClient = metrics.NewClient(params.MetricScope, service.GetMetricsServiceIdx(params.Name, params.Logger))

	clusterMetadata := s.cfg.ClusterMetadata
	params.ClusterMetadata = cluster.NewMetadata(
		params.Logger,
		dc.GetBoolProperty(dynamicconfig.EnableGlobalDomain, clusterMetadata.EnableGlobalDomain),
		clusterMetadata.FailoverVersionIncrement,
		clusterMetadata.MasterClusterName,
		clusterMetadata.CurrentClusterName,
		clusterMetadata.ClusterInformation,
	)

	if s.cfg.PublicClient.HostPort != "" {
		params.DispatcherProvider = client.NewDNSYarpcDispatcherProvider(params.Logger, s.cfg.PublicClient.RefreshInterval)
	} else {
		log.Fatalf("need to provide an endpoint config for PublicClient")
	}

	params.ESConfig = &s.cfg.ElasticSearch
	params.ESConfig.Enable = dc.GetBoolProperty(dynamicconfig.EnableVisibilityToKafka, params.ESConfig.Enable)() // force override with dynamic config
	if params.ClusterMetadata.IsGlobalDomainEnabled() {
		params.MessagingClient = messaging.NewKafkaClient(&s.cfg.Kafka, params.MetricsClient, zap.NewNop(), params.Logger, params.MetricScope, true, params.ESConfig.Enable)
	} else if params.ESConfig.Enable {
		params.MessagingClient = messaging.NewKafkaClient(&s.cfg.Kafka, params.MetricsClient, zap.NewNop(), params.Logger, params.MetricScope, false, params.ESConfig.Enable)
	} else {
		params.MessagingClient = nil
	}

	// enable visibility to kafka and enable visibility to elastic search are using one config
	if params.ESConfig.Enable {
		esClient, err := elasticsearch.NewClient(&s.cfg.ElasticSearch)
		if err != nil {
			log.Fatalf("error creating elastic search client: %v", err)
		}
		params.ESClient = esClient

		indexName, ok := params.ESConfig.Indices[common.VisibilityAppName]
		if !ok || len(indexName) == 0 {
			log.Fatalf("elastic search config missing visibility index")
		}
	}

	dispatcher, err := params.DispatcherProvider.Get(common.FrontendServiceName, s.cfg.PublicClient.HostPort)
	if err != nil {
		log.Fatalf("failed to construct dispatcher: %v", err)
	}
	params.PublicClient = workflowserviceclient.New(dispatcher.ClientConfig(common.FrontendServiceName))

	// Dynamic archival config is accessed once on cluster startup than never accessed again.
	// This is done so as to keep archival status and and the initialization of archiver.Provider in sync.
	// TODO: Once archival pause is implemented archival config can be made truly dynamic.
	clusterHistoryArchivalStatus := dc.GetStringProperty(dynamicconfig.HistoryArchivalStatus, s.cfg.Archival.History.Status)()
	enableReadFromHistoryArchival := dc.GetBoolProperty(dynamicconfig.EnableReadFromHistoryArchival, s.cfg.Archival.History.EnableRead)()
	clusterVisibilityArchivalStatus := dc.GetStringProperty(dynamicconfig.VisibilityArchivalStatus, s.cfg.Archival.Visibility.Status)()
	enableReadFromVisibilityArchival := dc.GetBoolProperty(dynamicconfig.EnableReadFromVisibilityArchival, s.cfg.Archival.Visibility.EnableRead)()
	params.ArchivalMetadata = archiver.NewArchivalMetadata(
		clusterHistoryArchivalStatus,
		enableReadFromHistoryArchival,
		clusterVisibilityArchivalStatus,
		enableReadFromVisibilityArchival,
		&s.cfg.DomainDefaults.Archival,
	)

	configuredForHistoryArchival := params.ArchivalMetadata.GetHistoryConfig().ClusterConfiguredForArchival()
	historyArchiverProviderCfg := s.cfg.Archival.History.Provider
	if (configuredForHistoryArchival && historyArchiverProviderCfg == nil) || (!configuredForHistoryArchival && historyArchiverProviderCfg != nil) {
		log.Fatalf("invalid history archival config")
	}

	configuredForVisibilityArchival := params.ArchivalMetadata.GetVisibilityConfig().ClusterConfiguredForArchival()
	visibilityArchiverProviderCfg := s.cfg.Archival.Visibility.Provider
	if (configuredForVisibilityArchival && visibilityArchiverProviderCfg == nil) || (!configuredForVisibilityArchival && visibilityArchiverProviderCfg != nil) {
		log.Fatalf("invalid visibility archival config")
	}
	params.ArchiverProvider = provider.NewArchiverProvider(historyArchiverProviderCfg, visibilityArchiverProviderCfg)

	params.PersistenceConfig.TransactionSizeLimit = dc.GetIntProperty(dynamicconfig.TransactionSizeLimit, common.DefaultTransactionSizeLimit)

	params.Logger.Info("Starting service " + s.name)

	var daemon common.Daemon

	switch s.name {
	case frontendService:
		daemon = frontend.NewService(&params)
	case historyService:
		daemon = history.NewService(&params)
	case matchingService:
		daemon = matching.NewService(&params)
	case workerService:
		daemon = worker.NewService(&params)
	}

	go execute(daemon, s.doneC)

	return daemon
}

// execute runs the daemon in a separate go routine
func execute(d common.Daemon, doneC chan struct{}) {
	d.Start()
	close(doneC)
}
