/*
Copyright 2020 Cortex Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operator

import (
	"github.com/cortexlabs/cortex/pkg/operator/config"
	"github.com/cortexlabs/cortex/pkg/types/clusterconfig"
	"github.com/cortexlabs/cortex/pkg/types/userconfig"
)

func addAPItoAPIGateway(loadBalancerScheme clusterconfig.LoadBalancerScheme, apiNetworking userconfig.APIGatewayType, apiName string) error {

	if loadBalancerScheme.String() == "internal" {
		if apiNetworking.String() == "public" {
			err := config.AWS.CreateRouteWithIntegration(apiName, config.Cluster.ClusterName)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func removeAPIfromAPIGateway(loadBalancerScheme clusterconfig.LoadBalancerScheme, apiName string) error {

	if loadBalancerScheme.String() == "internal" {
		err := config.AWS.DeleteAPIGatewayRoute(apiName, config.Cluster.ClusterName)
		if err != nil {
			return err
		}

	}

	return nil
}
