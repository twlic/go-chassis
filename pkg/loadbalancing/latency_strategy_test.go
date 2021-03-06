package loadbalancing_test

import (
	"sort"
	"testing"
	"time"

	"github.com/go-chassis/go-archaius"
	"github.com/go-chassis/go-chassis/v2/core/common"
	"github.com/go-chassis/go-chassis/v2/core/config"
	_ "github.com/go-chassis/go-chassis/v2/core/handler"
	"github.com/go-chassis/go-chassis/v2/core/invocation"
	"github.com/go-chassis/go-chassis/v2/core/loadbalancer"
	"github.com/go-chassis/go-chassis/v2/core/registry"
	"github.com/go-chassis/go-chassis/v2/pkg/loadbalancing"
	"github.com/go-chassis/go-chassis/v2/pkg/util/tags"
	"github.com/stretchr/testify/assert"
)

func TestWeightedResponseStrategy_Pick(t *testing.T) {
	archaius.Init(archaius.WithMemorySource())
	config.ReadLBFromArchaius()
	config.GetLoadBalancing().Strategy["name"] = loadbalancer.StrategyLatency
	instances := []*registry.MicroServiceInstance{
		{
			EndpointsMap: map[string]*registry.Endpoint{
				"rest": {
					false,
					"127.0.0.1:8080",
				},
				"highway": {
					false,
					"127.0.0.1:9090",
				},
			},
		},
		{
			EndpointsMap: map[string]*registry.Endpoint{
				"rest": {
					false,
					"10.0.0.3:8080",
				},
				"highway": {
					false,
					"10.0.0.3:9090",
				},
			},
		},
	}
	defaultTags := utiltags.Tags{}
	restTags := utiltags.Tags{Label: "app:mesh-app|version:latest"}
	loadbalancer.SetLatency(2*time.Second, "127.0.0.1:8080", "Server", restTags, common.ProtocolRest)
	loadbalancer.SetLatency(3*time.Second, "127.0.0.1:8080", "Server", restTags, common.ProtocolRest)
	loadbalancer.SetLatency(4*time.Second, "127.0.0.1:8080", "Server", restTags, common.ProtocolRest)

	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:8080", "Server", restTags, common.ProtocolRest)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:8080", "Server", restTags, common.ProtocolRest)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:8080", "Server", restTags, common.ProtocolRest)

	loadbalancer.SetLatency(1*time.Second, "127.0.0.1:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(3*time.Second, "127.0.0.1:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(5*time.Second, "127.0.0.1:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(9*time.Second, "127.0.0.1:9090", "Server", defaultTags, common.ProtocolHighway)

	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.3:9090", "Server", defaultTags, common.ProtocolHighway)
	loadbalancer.Enable(archaius.GetString("servicecomb.loadbalance.strategy.name", ""))
	f, _ := loadbalancer.GetStrategyPlugin(loadbalancer.StrategyLatency)
	s := f()
	inv := &invocation.Invocation{
		Protocol: common.ProtocolRest,
	}
	s.ReceiveData(inv, instances, "Server|app:mesh-app|version:latest")
	time.Sleep(31 * time.Second)
	var count int
	for i := 0; i < 100; i++ {
		instance, err := s.Pick()
		assert.NoError(t, err)
		if "10.0.0.3:8080" == instance.EndpointsMap["rest"].GenEndpoint() {
			count++
		}
	}
	t.Log(count)
	if !(count < 100) {
		t.Error(count)
	}
	s = f()
	inv = &invocation.Invocation{
		Protocol: common.ProtocolHighway,
	}
	s.ReceiveData(inv, instances, "Server")
	count = 0
	for i := 0; i < 1000; i++ {
		instance, err := s.Pick()
		assert.NoError(t, err)
		if "10.0.0.3:9090" == instance.EndpointsMap["highway"].GenEndpoint() {
			count++
		}

	}
	t.Log(count)
	if !(count < 1000) {
		t.Error(count)
	}
}
func TestCalculateAvgLatency(t *testing.T) {
	defaultTags := utiltags.Tags{}
	loadbalancer.SetLatency(2*time.Second, "127.0.0.1:3000", "Server1", defaultTags, common.ProtocolRest)
	loadbalancer.SetLatency(3*time.Second, "10.1.1.1.1:3000", "Server1", defaultTags, common.ProtocolRest)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.1:3000", "Server1", defaultTags, common.ProtocolRest)
	loadbalancer.SetLatency(1*time.Second, "127.0.0.1:5000", "Server1", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(5*time.Second, "127.0.0.1:5000", "Server1", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(1*time.Second, "10.0.0.1:5000", "Server1", defaultTags, common.ProtocolHighway)
	loadbalancer.SetLatency(9*time.Second, "10.0.0.1:5000", "Server1", defaultTags, common.ProtocolHighway)
	loadbalancing.CalculateAvgLatency()
	for k, v := range loadbalancer.ProtocolStatsMap {
		if k == loadbalancer.BuildKey("Server1", "", common.ProtocolRest) {
			for _, s := range v {
				if s.Addr == "127.0.0.1:3000" {
					assert.Equal(t, time.Duration(2*time.Second), s.AvgLatency)
				}
				if s.Addr == "10.1.1.1:3000" {
					assert.Equal(t, time.Duration(3*time.Second), s.AvgLatency)
				}
			}

		}
		if k == loadbalancer.BuildKey("Server1", "", common.ProtocolHighway) {
			for _, s := range v {
				if s.Addr == "127.0.0.1:5000" {
					assert.Equal(t, time.Duration(3*time.Second), s.AvgLatency)
				}
				if s.Addr == "10.0.0.1:5000" {
					assert.Equal(t, time.Duration(5*time.Second), s.AvgLatency)
				}
			}

		}
	}
	loadbalancing.SortLatency()
	t.Log(loadbalancer.ProtocolStatsMap)
	assert.Equal(t, "127.0.0.1:5000", loadbalancer.ProtocolStatsMap[loadbalancer.BuildKey("Server1", "", common.ProtocolHighway)][0].Addr)
	assert.Equal(t, "10.0.0.1:3000", loadbalancer.ProtocolStatsMap[loadbalancer.BuildKey("Server1", "", common.ProtocolRest)][0].Addr)
}
func TestSortLatency(t *testing.T) {
	p1 := &loadbalancer.ProtocolStats{
		AvgLatency: 1 * time.Second,
	}
	p2 := &loadbalancer.ProtocolStats{
		AvgLatency: 4 * time.Second,
	}
	p3 := &loadbalancer.ProtocolStats{
		AvgLatency: 2 * time.Second,
	}
	p4 := &loadbalancer.ProtocolStats{
		AvgLatency: 6 * time.Second,
	}
	s := []*loadbalancer.ProtocolStats{p1, p2, p3, p4}
	sort.Sort(loadbalancing.ByDuration(s))
	t.Log(s[0].AvgLatency)
	t.Log(s[1].AvgLatency)
	t.Log(s[2].AvgLatency)
	t.Log(s[3].AvgLatency)
}
