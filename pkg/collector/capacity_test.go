/*******************************************************************************
*
* Copyright 2017 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sapcc/go-bits/assert"
	"github.com/sapcc/limes/pkg/db"
	"github.com/sapcc/limes/pkg/limes"
	"github.com/sapcc/limes/pkg/test"
)

func Test_ScanCapacity(t *testing.T) {
	test.ResetTime()
	test.InitDatabase(t)

	cluster := &limes.Cluster{
		ID:              "west",
		IsServiceShared: map[string]bool{"shared": true},
		ServiceTypes:    []string{"shared", "unshared", "unshared2"},
		QuotaPlugins: map[string]limes.QuotaPlugin{
			"shared":    test.NewPlugin("shared"),
			"unshared":  test.NewPlugin("unshared"),
			"unshared2": test.NewPlugin("unshared2"),
		},
		CapacityPlugins: map[string]limes.CapacityPlugin{
			"unittest": test.NewCapacityPlugin("unittest",
				//publish capacity for some known resources...
				"shared/things",
				//...and some nonexistent ones (these should be ignored by the scraper)
				"whatever/things", "shared/items",
			),
			"unittest2": test.NewCapacityPlugin("unittest2",
				//same as above: some known...
				"unshared/capacity",
				//...and some unknown resources
				"someother/capacity",
			),
		},
		Config: &limes.ClusterConfiguration{Auth: &limes.AuthParameters{}},
	}

	c := Collector{
		Cluster:  cluster,
		Plugin:   nil,
		LogError: t.Errorf,
		TimeNow:  test.TimeNow,
	}

	//check that capacity records are created correctly (and that nonexistent
	//resources are ignored by the scraper)
	c.scanCapacity()
	test.AssertDBContent(t, "fixtures/scancapacity1.sql")

	//insert some crap records
	err := db.DB.Insert(&db.ClusterResource{
		ServiceID: 2,
		Name:      "unknown",
		Capacity:  100,
	})
	if err != nil {
		t.Error(err)
	}
	_, err = db.DB.Exec(
		`DELETE FROM cluster_resources WHERE service_id = ? AND name = ?`,
		1, "things",
	)
	if err != nil {
		t.Error(err)
	}

	//simulate manual maintenance of capacity value by user
	insertTime := test.TimeNow()
	err = db.DB.Insert(&db.ClusterService{
		ClusterID: "west",
		Type:      "unshared2",
		ScrapedAt: &insertTime,
	})
	if err != nil {
		t.Error(err)
	}
	err = db.DB.Insert(&db.ClusterResource{
		ServiceID: 1,
		Name:      "capacity",
		Capacity:  50,
		Comment:   "manual",
	})
	if err != nil {
		t.Error(err)
	}
	err = db.DB.Insert(&db.ClusterResource{
		ServiceID: 3,
		Name:      "capacity",
		Capacity:  50,
		Comment:   "manual",
	})
	if err != nil {
		t.Error(err)
	}

	test.AssertDBContent(t, "fixtures/scancapacity2.sql")

	//next scan should throw out the crap records and recreate the deleted ones,
	//but keep the manually maintained ones; also change the reported Capacity to
	//see if updates are getting through
	cluster.CapacityPlugins["unittest"].(*test.CapacityPlugin).Capacity = 23
	c.scanCapacity()
	test.AssertDBContent(t, "fixtures/scancapacity3.sql")

	//add another capacity plugin covering a resource that currently has a
	//manually maintained resource record; check that this resource is upgraded
	//to automatically maintained by the next scan run
	cluster.CapacityPlugins["unittest3"] = test.NewCapacityPlugin("unittest3", "shared/capacity")
	c.scanCapacity()
	test.AssertDBContent(t, "fixtures/scancapacity4.sql")

	//add a capacity plugin that reports subcapacities; check that subcapacities
	//are correctly written when creating a cluster_resources record
	subcapacityPlugin := test.NewCapacityPlugin("unittest4", "unshared/things")
	subcapacityPlugin.WithSubcapacities = true
	cluster.CapacityPlugins["unittest4"] = subcapacityPlugin
	c.scanCapacity()
	test.AssertDBContent(t, "fixtures/scancapacity5.sql")

	//check that scraping correctly updates subcapacities on an existing record
	subcapacityPlugin.Capacity = 10
	c.scanCapacity()
	test.AssertDBContent(t, "fixtures/scancapacity6.sql")

	//check data metrics generated for these capacity data
	registry := prometheus.NewPedanticRegistry()
	dmc := &DataMetricsCollector{Cluster: cluster}
	registry.MustRegister(dmc)
	assert.HTTPRequest{
		Method:           "GET",
		Path:             "/metrics",
		ExpectStatusCode: 200,
		ExpectFile:       "fixtures/capacity_metrics.prom",
	}.Check(t, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
}
