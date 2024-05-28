// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	vnetproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	teletermv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusteridcache"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/vnet"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "term:vnet")

type status int

const (
	statusNotRunning status = iota
	statusRunning
	statusClosed
)

// Service implements gRPC service for VNet.
type Service struct {
	api.UnimplementedVnetServiceServer

	cfg            Config
	mu             sync.Mutex
	status         status
	processManager *vnet.ProcessManager
}

// New creates an instance of Service.
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg: cfg,
	}, nil
}

type Config struct {
	// DaemonService is used to get cached clients and for usage reporting. If DaemonService was not
	// one giant blob of methods, Config could accept two separate services instead.
	DaemonService *daemon.Service
	// ClientStore is needed to extract api/profile.Profile from Connect's tsh dir. Technically it
	// could create Teleport clients as well, but we use daemon.Service for that instead since it
	// includes a bunch of teleterm-specific necessities.
	ClientStore *client.Store
	// InsecureSkipVerify signifies whether VNet is going to verify the identity of the proxy service.
	InsecureSkipVerify bool
	// ClusterIDCache is used for usage reporting to read cluster ID that needs to be included with
	// every event.
	ClusterIDCache *clusteridcache.Cache
	// InstallationID is a unique ID of this particular Connect installation, used for usage
	// reporting.
	InstallationID string
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.DaemonService == nil {
		return trace.BadParameter("missing DaemonService")
	}

	if c.ClientStore == nil {
		return trace.BadParameter("missing ClientStore")
	}

	if c.ClusterIDCache == nil {
		return trace.BadParameter("missing ClusterIDCache")
	}

	if c.InstallationID == "" {
		return trace.BadParameter("missing InstallationID")
	}

	return nil
}

func (s *Service) Start(ctx context.Context, req *api.StartRequest) (*api.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == statusClosed {
		return nil, trace.CompareFailed("VNet service has been closed")
	}

	if s.status == statusRunning {
		return &api.StartResponse{}, nil
	}

	appProvider := &appProvider{
		daemonService:      s.cfg.DaemonService,
		clientStore:        s.cfg.ClientStore,
		insecureSkipVerify: s.cfg.InsecureSkipVerify,
		usageReporter:      &disabledTelemetryUsageReporter{},
	}

	// Generally, the usage reporting setting cannot be changed without restarting the app, so
	// technically this information could have been passed through argv to tsh daemon.
	// However, there is one exception: during the first launch of the app, the user is asked if they
	// want to enable telemetry. Agreeing to that changes the setting without restarting the app.
	// As such, this service needs to ask for this setting on every launch.
	isUsageReportingEnabled, err := s.isUsageReportingEnabled(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting usage reporting settings")
	}

	if isUsageReportingEnabled {
		usageReporter, err := newDaemonUsageReporter(daemonUsageReporterConfig{
			ClientStore:    s.cfg.ClientStore,
			ClientCache:    s.cfg.DaemonService,
			EventConsumer:  s.cfg.DaemonService,
			ClusterIDCache: s.cfg.ClusterIDCache,
			InstallationID: s.cfg.InstallationID,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		appProvider.usageReporter = usageReporter
	}

	processManager, err := vnet.SetupAndRun(ctx, appProvider)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		err := processManager.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.ErrorContext(ctx, "VNet closed with an error", "error", err)
		} else {
			log.DebugContext(ctx, "VNet closed")
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		// Handle unexpected shutdown.
		// If processManager.Wait has returned but status is stil "running", then it means that VNet
		// unexpectedly shut down rather than stopped through the Stop RPC.
		if s.status == statusRunning {
			s.status = statusNotRunning

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			reportErr := s.reportUnexpectedShutdown(ctx, err)
			if reportErr != nil {
				log.ErrorContext(ctx, "Could not notify the Electron app about unexpected VNet shutdown",
					"shutdown_error", err, "notify_error", reportErr)
			}
		}
	}()

	s.processManager = processManager
	s.status = statusRunning
	return &api.StartResponse{}, nil
}

// Stop stops VNet and cleans up used resources. Blocks until VNet stops or ctx is canceled.
func (s *Service) Stop(ctx context.Context, req *api.StopRequest) (*api.StopResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.stopLocked()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.StopResponse{}, nil
}

func (s *Service) stopLocked() error {
	if s.status == statusClosed {
		return trace.CompareFailed("VNet service has been closed")
	}

	if s.status == statusNotRunning {
		return nil
	}

	s.processManager.Close()
	err := s.processManager.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return trace.Wrap(err)
	}

	s.status = statusNotRunning
	return nil
}

// Close stops VNet service and prevents it from being started again. Blocks until VNet stops.
// Intended for cleanup code when tsh daemon gets terminated.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.stopLocked()
	if err != nil {
		return trace.Wrap(err)
	}

	s.status = statusClosed
	return nil
}

func (s *Service) isUsageReportingEnabled(ctx context.Context) (bool, error) {
	tshdEventsClient, err := s.cfg.DaemonService.TshdEventsClient()
	if err != nil {
		return false, trace.Wrap(err)
	}

	resp, err := tshdEventsClient.GetUsageReportingSettings(ctx, &teletermv1.GetUsageReportingSettingsRequest{})
	if err != nil {
		return false, trace.Wrap(err)
	}

	return resp.UsageReportingSettings.Enabled, nil
}

func (s *Service) reportUnexpectedShutdown(ctx context.Context, shutdownErr error) error {
	tshdEventsClient, err := s.cfg.DaemonService.TshdEventsClient()
	if err != nil {
		return trace.Wrap(err, "obtaining tshd events client")
	}

	var shutdownErrorMsg string
	if shutdownErr != nil {
		shutdownErrorMsg = shutdownErr.Error()
	}

	_, err = tshdEventsClient.ReportUnexpectedVnetShutdown(ctx, &teletermv1.ReportUnexpectedVnetShutdownRequest{
		Error: shutdownErrorMsg,
	})
	return trace.Wrap(err, "sending shutdown report")
}

type appProvider struct {
	daemonService      *daemon.Service
	clientStore        *client.Store
	usageReporter      usageReporter
	insecureSkipVerify bool
}

func (p *appProvider) ListProfiles() ([]string, error) {
	profiles, err := p.clientStore.ListProfiles()
	return profiles, trace.Wrap(err)
}

func (p *appProvider) GetCachedClient(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	uri := uri.NewClusterURI(profileName).AppendLeafCluster(leafClusterName)
	client, err := p.daemonService.GetCachedClient(ctx, uri)
	return client, trace.Wrap(err)
}

func (p *appProvider) ReissueAppCert(ctx context.Context, profileName, leafClusterName string, app types.Application) (tls.Certificate, error) {
	clusterURI := uri.NewClusterURI(profileName).AppendLeafCluster(leafClusterName)
	cluster, _, err := p.daemonService.ResolveClusterURI(clusterURI)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	client, err := p.daemonService.GetCachedClient(ctx, clusterURI)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// TODO(ravicious): Copy stuff from DaemonService.reissueGatewayCerts in order to handle expired certs.
	cert, err := cluster.ReissueAppCert(ctx, client, app)
	return cert, trace.Wrap(err)
}

// GetDialOptions returns ALPN dial options for the profile.
func (p *appProvider) GetDialOptions(ctx context.Context, profileName string) (*vnet.DialOptions, error) {
	profile, err := p.clientStore.GetProfile(profileName)
	if err != nil {
		return nil, trace.Wrap(err, "loading user profile")
	}
	dialOpts := &vnet.DialOptions{
		WebProxyAddr:            profile.WebProxyAddr,
		ALPNConnUpgradeRequired: profile.TLSRoutingConnUpgradeRequired,
		InsecureSkipVerify:      p.insecureSkipVerify,
	}
	if dialOpts.ALPNConnUpgradeRequired {
		dialOpts.RootClusterCACertPool, err = p.getRootClusterCACertPool(ctx, profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return dialOpts, nil
}

func (p *appProvider) GetVnetConfig(ctx context.Context, profileName, leafClusterName string) (*vnetproto.VnetConfig, error) {
	clusterClient, err := p.GetCachedClient(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vnetConfigClient := clusterClient.AuthClient.VnetConfigServiceClient()
	vnetConfig, err := vnetConfigClient.GetVnetConfig(ctx, &vnetproto.GetVnetConfigRequest{})
	return vnetConfig, trace.Wrap(err)
}

// OnNewConnection submits a usage event once per appProvider lifetime.
// That is, if a user makes multiple connections to a single app, OnNewConnection submits a single
// event. This is to mimic how Connect submits events for its app gateways. This lets us compare
// popularity of VNet and app gateways.
func (p *appProvider) OnNewConnection(ctx context.Context, profileName, leafClusterName string, app types.Application) error {
	// Enqueue the event from a separate goroutine since we don't care about errors anyway and we also
	// don't want to slow down VNet connections.
	go func() {
		uri := uri.NewClusterURI(profileName).AppendLeafCluster(leafClusterName).AppendApp(app.GetName())

		err := p.usageReporter.ReportApp(ctx, uri)
		if err != nil {
			log.ErrorContext(ctx, "Failed to submit usage event", "app", uri, "error", err)
		}
	}()

	return nil
}

// getRootClusterCACertPool returns a certificate pool for the root cluster of the given profile.
func (p *appProvider) getRootClusterCACertPool(ctx context.Context, profileName string) (*x509.CertPool, error) {
	tc, err := p.newTeleportClient(ctx, profileName, "")
	if err != nil {
		return nil, trace.Wrap(err, "creating new client")
	}
	certPool, err := tc.RootClusterCACertPool(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading root cluster CA cert pool")
	}
	return certPool, nil
}

func (p *appProvider) newTeleportClient(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
	cfg := &client.Config{
		ClientStore: p.clientStore,
	}
	if err := cfg.LoadProfile(p.clientStore, profileName); err != nil {
		return nil, trace.Wrap(err, "loading client profile")
	}
	if leafClusterName != "" {
		cfg.SiteName = leafClusterName
	}
	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating new client")
	}
	return tc, nil
}

type usageReporter interface {
	ReportApp(context.Context, uri.ResourceURI) error
}

type daemonUsageReporter struct {
	cfg daemonUsageReporterConfig
	// reportedApps contains a set of URIs for apps which usage has been already reported.
	// App gateways (local proxies) in Connect report a single event per gateway created per app. VNet
	// needs to replicate this behavior, hence why it keeps track of reported apps to report only one
	// event per app per VNet's lifespan.
	reportedApps map[string]struct{}
	// mu protects access to reportedApps.
	mu sync.Mutex
}

type clientCache interface {
	GetCachedClient(context.Context, uri.ResourceURI) (*client.ClusterClient, error)
}

type eventConsumer interface {
	ReportUsageEvent(*teletermv1.ReportUsageEventRequest) error
}

type daemonUsageReporterConfig struct {
	ClientStore   *client.Store
	ClientCache   clientCache
	EventConsumer eventConsumer
	// clusterIDCache stores cluster ID that needs to be included with each usage event. It's updated
	// outside of usageReporter – the middleware merely reads data from it. If the cache does not
	// contain the given cluster ID, usageReporter drops the event.
	ClusterIDCache *clusteridcache.Cache
	InstallationID string
}

func (c *daemonUsageReporterConfig) CheckAndSetDefaults() error {
	if c.ClientStore == nil {
		return trace.BadParameter("missing ClientStore")
	}

	if c.ClientCache == nil {
		return trace.BadParameter("missing ClientCache")
	}

	if c.EventConsumer == nil {
		return trace.BadParameter("missing EventConsumer")
	}

	if c.ClusterIDCache == nil {
		return trace.BadParameter("missing ClusterIDCache")
	}

	if c.InstallationID == "" {
		return trace.BadParameter("missing InstallationID")
	}

	return nil
}

func newDaemonUsageReporter(cfg daemonUsageReporterConfig) (*daemonUsageReporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &daemonUsageReporter{
		cfg:          cfg,
		reportedApps: make(map[string]struct{}),
	}, nil
}

func (r *daemonUsageReporter) ReportApp(ctx context.Context, appURI uri.ResourceURI) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, hasAppBeenReported := r.reportedApps[appURI.String()]; hasAppBeenReported {
		log.DebugContext(ctx, "App was already reported", "app", appURI.String())
		return nil
	}

	rootClusterURI := appURI.GetRootClusterURI()
	client, err := r.cfg.ClientCache.GetCachedClient(ctx, appURI)
	if err != nil {
		return trace.Wrap(err)
	}
	rootClusterName := client.RootClusterName()
	profile, err := r.cfg.ClientStore.GetProfile(appURI.GetProfileName())
	if err != nil {
		return trace.Wrap(err)
	}

	clusterID, ok := r.cfg.ClusterIDCache.Load(rootClusterURI)
	if !ok {
		return trace.NotFound("cluster ID for %q not found", rootClusterURI)
	}

	log.DebugContext(ctx, "Reporting usage event", "app", appURI.String())

	err = r.cfg.EventConsumer.ReportUsageEvent(&teletermv1.ReportUsageEventRequest{
		AuthClusterId: clusterID,
		PrehogReq: &prehogv1alpha.SubmitConnectEventRequest{
			DistinctId: r.cfg.InstallationID,
			Timestamp:  timestamppb.Now(),
			Event: &prehogv1alpha.SubmitConnectEventRequest_ProtocolUse{
				ProtocolUse: &prehogv1alpha.ConnectProtocolUseEvent{
					ClusterName:   rootClusterName,
					UserName:      profile.Username,
					Protocol:      "app",
					Origin:        "vnet",
					AccessThrough: "vnet",
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "adding usage event to queue")
	}

	r.reportedApps[appURI.String()] = struct{}{}

	return nil
}

type disabledTelemetryUsageReporter struct{}

func (r *disabledTelemetryUsageReporter) ReportApp(ctx context.Context, appURI uri.ResourceURI) error {
	log.DebugContext(ctx, "Skipping usage event, usage reporting is turned off", "app", appURI.String())
	return nil
}
