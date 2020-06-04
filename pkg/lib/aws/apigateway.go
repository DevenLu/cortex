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

package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/apigatewayv2"
	"github.com/cortexlabs/cortex/pkg/lib/errors"
)

// GetVPCLinkByTag Gets a VPC Link by tag (returns nil if there are no matches)
func (c *Client) GetVPCLinkByTag(tagName string, tagValue string) (*apigatewayv2.VpcLink, error) {
	vpcLinks, err := c.APIGatewayV2().GetVpcLinks(&apigatewayv2.GetVpcLinksInput{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vpc links")
	}

	for {
		for _, vpcLink := range vpcLinks.Items {
			for tag, value := range vpcLink.Tags {
				if tag == tagName && *value == tagValue {
					return vpcLink, nil
				}
			}
		}
		// next token nil means no more pages of VPC links
		if vpcLinks.NextToken == nil {
			break
		}
		vpcLinks, err = c.APIGatewayV2().GetVpcLinks(&apigatewayv2.GetVpcLinksInput{
			NextToken: vpcLinks.NextToken,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get vpc links")
		}
	}

	return nil, nil
}

// GetAPIGatewayByTag Gets an API Gateway by tag (returns nil if there are no matches)
func (c *Client) GetAPIGatewayByTag(tagName string, tagValue string) (*apigatewayv2.Api, error) {
	apis, err := c.APIGatewayV2().GetApis(&apigatewayv2.GetApisInput{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get api gateways")
	}

	for {
		for _, api := range apis.Items {
			for tag, value := range api.Tags {
				if tag == tagName && *value == tagValue {
					return api, nil
				}
			}
		}
		if apis.NextToken == nil {
			break
		}
		apis, err = c.APIGatewayV2().GetApis(&apigatewayv2.GetApisInput{
			NextToken: apis.NextToken,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get api gateways")
		}
	}

	return nil, nil
}

// DeleteVPCLinkByTag Deletes a VPC Link by tag (returns whether or not the VPC Link existed)
func (c *Client) DeleteVPCLinkByTag(tagName string, tagValue string) (bool, error) {
	vpcLink, err := c.GetVPCLinkByTag(tagName, tagValue)
	if err != nil {
		return false, err
	} else if vpcLink == nil {
		return false, nil
	}

	_, err = c.APIGatewayV2().DeleteVpcLink(&apigatewayv2.DeleteVpcLinkInput{
		VpcLinkId: vpcLink.VpcLinkId,
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to delete vpc link")
	}

	return true, nil
}

// DeleteAPIGatewayByTag Deletes an API Gateway by tag (returns whether or not the API Gateway existed)
func (c *Client) DeleteAPIGatewayByTag(tagName string, tagValue string) (bool, error) {
	apiGateway, err := c.GetAPIGatewayByTag(tagName, tagValue)
	if err != nil {
		return false, err
	} else if apiGateway == nil {
		return false, nil
	}

	_, err = c.APIGatewayV2().DeleteApi(&apigatewayv2.DeleteApiInput{
		ApiId: apiGateway.ApiId,
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to delete api gateway")
	}

	return true, nil
}

// GetVPCLinkIntegration gets the VPC Link integration in an API Gateway, or nil if unable to find it
func (c *Client) GetVPCLinkIntegration(apiGatewayID string, vpcLinkID string) (*apigatewayv2.Integration, error) {
	integrations, err := c.APIGatewayV2().GetIntegrations(&apigatewayv2.GetIntegrationsInput{
		ApiId: &apiGatewayID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get api gateway integrations")
	}

	// find integration which is connected to the VPC link
	for _, integration := range integrations.Items {
		if *integration.ConnectionId == vpcLinkID {
			return integration, nil
		}
	}

	return nil, nil
}

// GetRouteIntegrationID returns the integration which is attached to a endpoint route, or empty string if unable to find it
func (c *Client) GetRouteIntegrationID(apiGatewayID string, endpoint string) (string, error) {
	route, err := c.GetRoute(apiGatewayID, endpoint)
	if err != nil {
		return "", err
	}

	// trim of prefix of integrationID.
	// Note: Integrations get attached to routes via a target of the format integrations/<integrationID>
	integrationID := strings.Trim(*route.Target, "integrations/")
	return integrationID, nil
}

// GetRoute retrieves the route matching an endpoint, or nil if unable to find it
func (c *Client) GetRoute(apiGatewayID string, endpoint string) (*apigatewayv2.Route, error) {
	routes, err := c.APIGatewayV2().GetRoutes(&apigatewayv2.GetRoutesInput{
		ApiId: &apiGatewayID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get api gateway routes")
	}

	// find route which matches the endpoint
	for _, route := range routes.Items {
		if *route.RouteKey == "ANY "+endpoint {
			return route, nil
		}
	}

	return nil, nil
}

// CreateRoute creates a new route and attaches the route to the integration
func (c *Client) CreateRoute(apiGatewayID string, integrationID string, endpoint string) error {
	_, err := c.APIGatewayV2().CreateRoute(&apigatewayv2.CreateRouteInput{
		ApiId:    &apiGatewayID,
		RouteKey: aws.String("ANY " + endpoint),
		Target:   aws.String("integrations/" + integrationID),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create route for api gateway")
	}
	return nil
}

// CreateHTTPIntegration creates new HTTP integration for API Gateway, returns integration ID
func (c *Client) CreateHTTPIntegration(apiGatewayID string, targetEndpoint string) (string, error) {
	integrationResponse, err := c.APIGatewayV2().CreateIntegration(&apigatewayv2.CreateIntegrationInput{
		ApiId:                &apiGatewayID,
		IntegrationType:      aws.String("HTTP_PROXY"),
		IntegrationUri:       &targetEndpoint,
		PayloadFormatVersion: aws.String("1.0"),
		IntegrationMethod:    aws.String("ANY"),
	})
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to create api gateway integration for endpoint %s", targetEndpoint))
	}
	return *integrationResponse.IntegrationId, nil
}

// DeleteIntegration deletes an integration from API Gateway
func (c *Client) DeleteIntegration(apiGatewayID string, integrationID string) error {
	_, err := c.APIGatewayV2().DeleteIntegration(&apigatewayv2.DeleteIntegrationInput{
		ApiId:         &apiGatewayID,
		IntegrationId: &integrationID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete api gateway integration")
	}
	return nil
}

// DeleteRoute deletes a route from API Gateway, and whether the route was found
func (c *Client) DeleteRoute(apiGatewayID string, endpoint string) (bool, error) {
	route, err := c.GetRoute(apiGatewayID, endpoint)
	if err != nil {
		return false, err
	} else if route == nil {
		return false, nil
	}

	_, err = c.APIGatewayV2().DeleteRoute(&apigatewayv2.DeleteRouteInput{
		ApiId:   &apiGatewayID,
		RouteId: route.RouteId,
	})
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to delete api gateway route with endpoint %s", endpoint))
	}

	return true, nil
}
