package vnet

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/trace"
)

type ProfileStore interface {
	ListProfiles() ([]string, error)
	GetProfile(string) (*profile.Profile, error)
}

type TCPAppResolverConfig struct {
	ProfileStore  ProfileStore
	ClientCache   *clientcache.Cache
	NewClientFunc clientcache.NewClientFunc
}

type TCPAppResolver struct {
	cfg *TCPAppResolverConfig
}

func NewTCPAppResolver(cfg *TCPAppResolverConfig) (*TCPAppResolver, error) {
	return &TCPAppResolver{
		cfg: cfg,
	}, nil
}

func (r *TCPAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (handler TCPHandler, match bool, err error) {
	profileNames, err := r.cfg.ProfileStore.ListProfiles()
	if err != nil {
		return nil, false, trace.Wrap(err, "listing profiles")
	}
	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	for _, profileName := range profileNames {
		if !isSubdomain(fqdn, profileName) {
			// TODO(nklaassen): handle custom DNS zones and leaf clusters.
			continue
		}
		leafClusterName := ""

		clusterClient, err := r.cfg.ClientCache.Get(ctx, profileName, leafClusterName)
		if err != nil {
			return nil, false, trace.Wrap(err, "getting client from cache")
		}

		appServers, err := apiclient.GetAllResources[types.AppServer](ctx, clusterClient.AuthClient, &proto.ListResourcesRequest{
			ResourceType:        types.KindAppServer,
			PredicateExpression: fmt.Sprintf(`resource.spec.public_addr == "%s" && hasPrefix(resource.spec.uri, "tcp://")`, appPublicAddr),
		})
		if err != nil {
			return nil, false, trace.Wrap(err, "listing application servers")
		}

		for _, appServer := range appServers {
			app := appServer.GetApp()
			if app.GetPublicAddr() == appPublicAddr && app.IsTCP() {
				appHandler, err := newTCPAppHandler(ctx, r, clusterClient, app, profileName, leafClusterName)
				if err != nil {
					return nil, false, trace.Wrap(err)
				}
				return appHandler, true, nil
			}
		}
	}
	return nil, false, nil
}

type tcpAppHandler struct {
	app             types.Application
	NewClientFunc   clientcache.NewClientFunc
	clientCache     *clientcache.Cache
	profileName     string
	leafClusterName string
	lp              *alpnproxy.LocalProxy
}

func newTCPAppHandler(
	ctx context.Context,
	r *TCPAppResolver,
	clusterClient *client.ClusterClient,
	app types.Application,
	profileName string,
	leafClusterName string,
) (*tcpAppHandler, error) {
	teleportClient, err := r.cfg.NewClientFunc(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err, "creating Teleport client")
	}

	localProxyConfig := alpnproxy.LocalProxyConfig{
		ParentContext:           ctx,
		InsecureSkipVerify:      teleportClient.InsecureSkipVerify,
		RemoteProxyAddr:         teleportClient.WebProxyAddr,
		ALPNConnUpgradeRequired: teleportClient.TLSRoutingConnUpgradeRequired,
	}

	h := &tcpAppHandler{
		app:             app,
		NewClientFunc:   r.cfg.NewClientFunc,
		clientCache:     r.cfg.ClientCache,
		profileName:     profileName,
		leafClusterName: leafClusterName,
	}
	appCerts, err := h.reissueAppCert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen/ravicious): add middleware to refresh expired app certs and handle per-session MFA.
	lp, err := alpnproxy.NewLocalProxy(
		localProxyConfig,
		alpnproxy.WithALPNProtocol(alpncommon.ProtocolTCP),
		alpnproxy.WithClientCerts(appCerts),
		alpnproxy.WithClusterCAsIfConnUpgrade(ctx, teleportClient.RootClusterCACertPool),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}

	h.lp = lp
	return h, nil
}

func (h *tcpAppHandler) HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error {
	return trace.Wrap(h.lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

func (h *tcpAppHandler) reissueAppCert(ctx context.Context) (tls.Certificate, error) {
	teleportClient, err := h.NewClientFunc(ctx, h.profileName, h.leafClusterName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "creating Teleport client")
	}
	status, err := teleportClient.ProfileStatus()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "loading profile status")
	}

	routeToApp := proto.RouteToApp{
		Name:        h.app.GetName(),
		PublicAddr:  h.app.GetPublicAddr(),
		ClusterName: teleportClient.SiteName,
	}

	// TODO (Joerger): DELETE IN v17.0.0
	rootClient, err := h.clientCache.Get(ctx, h.profileName, "" /*leafClusterName*/)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting root cluster client")
	}
	routeToApp.SessionID, err = auth.TryCreateAppSessionForClientCertV15(ctx, rootClient.AuthClient, status.Username, routeToApp)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	clusterClient := rootClient
	if h.leafClusterName != "" {
		clusterClient, err = h.clientCache.Get(ctx, h.profileName, h.leafClusterName)
		return tls.Certificate{}, trace.Wrap(err, "getting leaf cluster client")
	}
	err = clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: teleportClient.SiteName,
		RouteToApp:     routeToApp,
		AccessRequests: status.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	key, err := teleportClient.LocalAgent().GetKey(teleportClient.SiteName, client.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, ok := key.AppTLSCerts[h.app.GetName()]
	if !ok {
		return tls.Certificate{}, trace.NotFound("the user is not logged in into the application %v", h.app.GetName())
	}

	tlsCert, err := key.TLSCertificate(cert)
	return tlsCert, trace.Wrap(err)
}

func isSubdomain(appFQDN, proxyAddress string) bool {
	// Fully-qualify the proxy address
	if !strings.HasSuffix(proxyAddress, ".") {
		proxyAddress = proxyAddress + "."
	}
	return strings.HasSuffix(appFQDN, "."+proxyAddress)
}
