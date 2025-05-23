/*******************************************************************************
*
* Copyright 2024 SAP SE
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

package datamodel

import (
	"fmt"
	"testing"

	"github.com/sapcc/go-api-declarations/liquid"
	"github.com/sapcc/go-bits/assert"
	"github.com/sapcc/go-bits/easypg"

	"github.com/sapcc/limes/internal/db"
	"github.com/sapcc/limes/internal/test"
)

func TestMain(m *testing.M) {
	easypg.WithTestDB(m, func() int { return m.Run() })
}

const (
	testQuotaOverridesNoRenamingConfigYAML = `
		availability_zones: [ az-one, az-two ]
		discovery:
			method: --test-static
		liquids:
			first:
				area: first
				liquid_service_type: %[1]s
			second:
				area: second
				liquid_service_type: %[1]s
	`

	testQuotaOverridesWithRenamingConfigYAML = `
		availability_zones: [ az-one, az-two ]
		discovery:
			method: --test-static
		liquids:
			first:
				area: first
				liquid_service_type: %[1]s
			second:
				area: second
				liquid_service_type: %[1]s
		resource_behavior:
		- resource: first/capacity
			identity_in_v1_api: capacities/first
		- resource: (first)/thi(ngs)
			identity_in_v1_api: thi$2/$1
	`
)

var expectedQuotaOverrides = map[string]map[string]map[db.ServiceType]map[liquid.ResourceName]uint64{
	"firstdomain": {
		"firstproject": {
			"first": {
				"capacity": 10,
				"things":   20,
			},
			"second": {
				"capacity": 30,
				"things":   40,
			},
		},
		"secondproject": {
			"first": {
				"capacity": 50,
			},
			"second": {
				"capacity": 60,
			},
		},
	},
	"seconddomain": {
		"thirdproject": {
			"first": {
				"things": 70,
			},
			"second": {
				"things": 80,
			},
		},
	},
}

func TestQuotaOverridesWithoutResourceRenaming(t *testing.T) {
	t.Setenv("LIMES_QUOTA_OVERRIDES_PATH", "fixtures/quota-overrides-no-renaming.json")
	srvInfo := test.DefaultLiquidServiceInfo()
	_, liquidServiceType := test.NewMockLiquidClient(srvInfo)
	s := test.NewSetup(t,
		test.WithConfig(fmt.Sprintf(testQuotaOverridesNoRenamingConfigYAML, liquidServiceType)),
	)
	overrides, errs := LoadQuotaOverrides(s.Cluster)
	for _, err := range errs {
		t.Error(err.Error())
	}
	assert.DeepEqual(t, "quota overrides", overrides, expectedQuotaOverrides)
}

func TestQuotaOverridesWithResourceRenaming(t *testing.T) {
	t.Setenv("LIMES_QUOTA_OVERRIDES_PATH", "fixtures/quota-overrides-with-renaming.json")
	srvInfo := test.DefaultLiquidServiceInfo()
	_, liquidServiceType := test.NewMockLiquidClient(srvInfo)
	s := test.NewSetup(t,
		test.WithConfig(fmt.Sprintf(testQuotaOverridesWithRenamingConfigYAML, liquidServiceType)),
	)
	overrides, errs := LoadQuotaOverrides(s.Cluster)
	for _, err := range errs {
		t.Error(err.Error())
	}
	assert.DeepEqual(t, "quota overrides", overrides, expectedQuotaOverrides)
}
