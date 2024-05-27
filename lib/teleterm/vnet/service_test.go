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
	"fmt"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/profile"
	teletermv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusteridcache"
)

func TestUsageReporter(t *testing.T) {
	eventConsumer := fakeEventConsumer{}

	validCluster := uri.NewClusterURI("foo")
	clusterWithoutClient := uri.NewClusterURI("no-client")
	clusterWithoutProfile := uri.NewClusterURI("no-profile")
	clusterWithoutClusterID := uri.NewClusterURI("no-cluster-id")

	clientCache := fakeClientCache{
		validClusterURIs: map[uri.ResourceURI]struct{}{
			validCluster:            struct{}{},
			clusterWithoutProfile:   struct{}{},
			clusterWithoutClusterID: struct{}{},
		},
	}

	clientStore := client.NewMemClientStore()
	err := clientStore.SaveProfile(&profile.Profile{
		WebProxyAddr: fmt.Sprintf("%s:3080", validCluster.GetProfileName()),
	}, false /* setCurrent */)
	require.NoError(t, err)
	err = clientStore.SaveProfile(&profile.Profile{
		WebProxyAddr: fmt.Sprintf("%s:3080", clusterWithoutClusterID.GetProfileName()),
	}, false /* setCurrent */)
	require.NoError(t, err)

	clusterIDcache := clusteridcache.Cache{}
	clusterIDcache.Store(uri.NewClusterURI("foo"), "1234")

	usageReporter, err := NewUsageReporter(UsageReporterConfig{
		EventConsumer:  &eventConsumer,
		ClientCache:    &clientCache,
		ClientStore:    clientStore,
		ClusterIDCache: &clusterIDcache,
		InstallationID: "4321",
	})
	require.NoError(t, err)
	t.Cleanup(usageReporter.Stop)

	// Verify that reporting the same app twice adds only one usage event.
	err = usageReporter.ReportApp(validCluster.AppendApp("app"))
	require.NoError(t, err)
	err = usageReporter.ReportApp(validCluster.AppendApp("app"))
	require.NoError(t, err)
	require.Equal(t, 1, eventConsumer.EventCount())

	// Verify that reporting an invalid cluster doesn't submit an event.
	err = usageReporter.ReportApp(clusterWithoutClient.AppendApp("bar"))
	require.True(t, trace.IsNotFound(err), "Not a NotFound error: %#v", err)
	require.Equal(t, 1, eventConsumer.EventCount())
	err = usageReporter.ReportApp(clusterWithoutProfile.AppendApp("bar"))
	require.True(t, trace.IsNotFound(err), "Not a NotFound error: %#v", err)
	require.Equal(t, 1, eventConsumer.EventCount())
	err = usageReporter.ReportApp(clusterWithoutClusterID.AppendApp("bar"))
	require.ErrorIs(t, err, trace.NotFound("cluster ID for \"/clusters/no-cluster-id\" not found"))
	require.Equal(t, 1, eventConsumer.EventCount())
}

func TestUsageReporter_Stop(t *testing.T) {
	eventConsumer := fakeEventConsumer{}
	clientCache := fakeClientCache{blockOnCtx: true}
	clientStore := client.NewMemClientStore()
	clusterIDCache := clusteridcache.Cache{}

	usageReporter, err := NewUsageReporter(UsageReporterConfig{
		EventConsumer:  &eventConsumer,
		ClientCache:    &clientCache,
		ClientStore:    clientStore,
		ClusterIDCache: &clusterIDCache,
		InstallationID: "4321",
	})
	require.NoError(t, err)
	t.Cleanup(usageReporter.Stop)

	go func() {
		usageReporter.Stop()
	}()

	uri := uri.NewClusterURI("foo").AppendApp("bar")
	err = usageReporter.ReportApp(uri)
	require.ErrorIs(t, err, context.Canceled)

	err = usageReporter.ReportApp(uri)
	require.True(t, trace.IsCompareFailed(err), "expected trace.CompareFailed but got %v", err)
}

type fakeEventConsumer struct {
	mu     sync.Mutex
	events []*teletermv1.ReportUsageEventRequest
}

func (ec *fakeEventConsumer) ReportUsageEvent(event *teletermv1.ReportUsageEventRequest) error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.events = append(ec.events, event)
	return nil
}

func (ec *fakeEventConsumer) EventCount() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	return len(ec.events)
}

type fakeClientCache struct {
	validClusterURIs map[uri.ResourceURI]struct{}
	// blockOnCtx makes GetCachedClient block until ctx is canceled.
	blockOnCtx bool
}

func (c *fakeClientCache) GetCachedClient(ctx context.Context, appURI uri.ResourceURI) (*client.ClusterClient, error) {
	if c.blockOnCtx {
		<-ctx.Done()
		return nil, trace.Wrap(ctx.Err())
	}

	if _, ok := c.validClusterURIs[appURI.GetClusterURI()]; !ok {
		return nil, trace.NotFound("client for cluster %q not found", appURI.GetClusterURI())
	}

	return &client.ClusterClient{}, nil
}
