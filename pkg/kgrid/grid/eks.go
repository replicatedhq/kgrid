package grid

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	smithy "github.com/aws/smithy-go"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

func isEKSNotFound(err error) bool {
	err = errors.Cause(err)

	smErr, ok := err.(*smithy.OperationError)
	if !ok {
		return false
	}

	httpErr, ok := smErr.Err.(*awshttp.ResponseError)
	if !ok {
		return false
	}

	_, ok = httpErr.ResponseError.Err.(*ekstypes.ResourceNotFoundException)
	return ok
}

func GetEKSClusterKubeConfig(region string, accessKeyID string, secretAccessKey string, clusterName string) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return "", errors.Wrap(err, "failed to load aws config")
	}
	cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	svc := eks.NewFromConfig(cfg)
	result, err := svc.DescribeCluster(context.Background(), &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to describe cluster")
	}

	b := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    server: %s
    certificate-authority-data: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
kind: Config
preferences: {}
users:
- name: aws
  user:
    exec:
        apiVersion: client.authentication.k8s.io/v1alpha1
        command: aws
        args:
        - "eks"
        - "get-token"
        - "--cluster-name"
        - "%s"
        env:
        - name: AWS_ACCESS_KEY_ID
          value: %s
        - name: AWS_SECRET_ACCESS_KEY
          value: %s
`, *result.Cluster.Endpoint, *result.Cluster.CertificateAuthority.Data, clusterName, accessKeyID, secretAccessKey)

	return b, nil
}

func GetEKSClusterNodePoolIsReady(region string, accessKeyID string, secretAccessKey string, clusterName string) (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return false, errors.Wrap(err, "failed to load aws config")
	}
	cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	svc := eks.NewFromConfig(cfg)
	result, err := svc.DescribeNodegroup(context.Background(), &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(clusterName),
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to describe cluster nodegroup")
	}

	return result.Nodegroup.Status == ekstypes.NodegroupStatusActive, nil
}

// getEKSClusterIsReady will return a bool if the cluster is completely ready for workloads
// we look at the cluster status in the AWS response to be "active"
func getEKSClusterIsReady(region string, accessKeyID string, secretAccessKey string, clusterName string) (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return false, errors.Wrap(err, "failed to load aws config")
	}
	cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	svc := eks.NewFromConfig(cfg)
	result, err := svc.DescribeCluster(context.Background(), &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to describe cluster")
	}

	return result.Cluster.Status == ekstypes.ClusterStatusActive, nil
}

// ensureEKSCluster will create the deterministic vpc for our clusters
func ensureEKSClusterVPC(cfg aws.Config) (*types.AWSVPC, error) {
	vpc := types.AWSVPC{}

	// all clusters end ip in a single VPC with a tag "replicatedhq-kubectl-grid=1"
	// look for this vpc and create if missing
	svc := ec2.NewFromConfig(cfg)

	describeVPCsInput := &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeVPCsResult, err := svc.DescribeVpcs(context.Background(), describeVPCsInput)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe VPCs")
	}
	if len(describeVPCsResult.Vpcs) > 0 {
		vpc.ID = *describeVPCsResult.Vpcs[0].VpcId
	} else {
		// create the vpc
		createVPCInput := &ec2.CreateVpcInput{
			CidrBlock: aws.String("172.24.0.0/16"),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeVpc,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("replicatedhq/kubectl-grid"),
							Value: aws.String("1"),
						},
					},
				},
			},
		}
		createVPCResult, err := svc.CreateVpc(context.Background(), createVPCInput)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create VPC")
		}

		vpc.ID = *createVPCResult.Vpc.VpcId
	}

	igwID, err := ensureInternetGateway(cfg, vpc.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure internet gateway")
	}
	vpc.InternetGatewayID = igwID

	securityGroupID, err := ensureEKSClusterSecurityGroup(cfg, vpc.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure security group")
	}
	vpc.SecurityGroupIDs = []string{
		securityGroupID,
	}

	privateSubnetIDs, err := ensurePrivateEKSSubnets(cfg, vpc.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure private subnets")
	}
	vpc.PrivateSubnetIDs = privateSubnetIDs

	publicSubnetID, err := ensurePublicEKSSubnet(cfg, vpc.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure public subnets")
	}
	vpc.PublicSubnetID = publicSubnetID

	err = ensurePublicSubnetRouteTable(cfg, vpc.ID, vpc.PublicSubnetID, vpc.InternetGatewayID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure public subnet route table")
	}

	eipAllocationID, err := ensureElasticIP(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure elastic ip")
	}
	vpc.EIPAllocationID = eipAllocationID

	natGatewayID, err := ensureNATGateway(cfg, vpc.PublicSubnetID, vpc.EIPAllocationID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure nat gateway")
	}
	vpc.NATGatewayID = natGatewayID

	for _, subnetID := range vpc.PrivateSubnetIDs {
		err = ensurePrivateSubnetRouteTable(cfg, vpc.ID, subnetID, vpc.NATGatewayID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to ensure private subnet route table")
		}
	}

	roleArn, err := ensureEKSRoleARN(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure role arn")
	}
	vpc.RoleArn = roleArn

	return &vpc, nil
}

func ensureInternetGateway(cfg aws.Config, vpcID string) (string, error) {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	describeInternetGatewaysInput := &ec2.DescribeInternetGatewaysInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeInternetGatewaysResult, err := svc.DescribeInternetGateways(ctx, describeInternetGatewaysInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to describe internet gateways")
	}
	if len(describeInternetGatewaysResult.InternetGateways) > 0 {
		return *describeInternetGatewaysResult.InternetGateways[0].InternetGatewayId, nil
	}

	createInternetGatewayInput := &ec2.CreateInternetGatewayInput{
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInternetGateway,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("replicatedhq/kubectl-grid"),
						Value: aws.String("1"),
					},
				},
			},
		},
	}

	createInternetGatewayResult, err := svc.CreateInternetGateway(ctx, createInternetGatewayInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to create internet gateway")
	}

	attachInternetGatewayInput := &ec2.AttachInternetGatewayInput{
		InternetGatewayId: createInternetGatewayResult.InternetGateway.InternetGatewayId,
		VpcId:             aws.String(vpcID),
	}
	_, err = svc.AttachInternetGateway(ctx, attachInternetGatewayInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to attach internet gateway to vpc")
	}

	return *createInternetGatewayResult.InternetGateway.InternetGatewayId, nil
}

func ensureEKSClusterSecurityGroup(cfg aws.Config, vpcID string) (string, error) {
	svc := ec2.NewFromConfig(cfg)

	describeSecurityGroupsInput := &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeSecurityGroupsResult, err := svc.DescribeSecurityGroups(context.Background(), describeSecurityGroupsInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to describe security groups")
	}
	if len(describeSecurityGroupsResult.SecurityGroups) > 0 {
		return *describeSecurityGroupsResult.SecurityGroups[0].GroupId, nil
	}

	createSecurityGroupInput := &ec2.CreateSecurityGroupInput{
		Description: aws.String("replicatedhq kubectl-grid"),
		GroupName:   aws.String("replicatedhq-kubectl-grid-default"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSecurityGroup,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("replicatedhq/kubectl-grid"),
						Value: aws.String("1"),
					},
				},
			},
		},
	}
	createSecurityGroupResult, err := svc.CreateSecurityGroup(context.Background(), createSecurityGroupInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to create security group")
	}

	return *createSecurityGroupResult.GroupId, nil
}

func ensurePrivateEKSSubnets(cfg aws.Config, vpcID string) ([]string, error) {
	svc := ec2.NewFromConfig(cfg)

	describeSubnetsInput := &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
					"replicatedhq/private",
				},
			},
		},
	}
	describeSubnetsResult, err := svc.DescribeSubnets(context.Background(), describeSubnetsInput)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe subnets")
	}

	subnetIDs := []string{}
	for _, subnet := range describeSubnetsResult.Subnets {
		for _, tag := range subnet.Tags {
			if tag.Key != nil && *tag.Key == "replicatedhq/private" {
				subnetIDs = append(subnetIDs, *subnet.SubnetId)
			}
		}
	}

	if len(subnetIDs) > 0 {
		// this is rough, if any succeed, it will return that list
		return subnetIDs, nil
	}

	subnetID, err := createSubnetInVPC(cfg, vpcID, "172.24.100.0/24", "us-west-1a", "replicatedhq/private")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create subnet")
	}
	subnetIDs = append(subnetIDs, subnetID)

	subnetID, err = createSubnetInVPC(cfg, vpcID, "172.24.101.0/24", "us-west-1b", "replicatedhq/private")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create subnet")
	}
	subnetIDs = append(subnetIDs, subnetID)

	return subnetIDs, nil
}

func ensurePublicEKSSubnet(cfg aws.Config, vpcID string) (string, error) {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	describeSubnetsInput := &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeSubnetsResult, err := svc.DescribeSubnets(ctx, describeSubnetsInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to describe subnets")
	}
	for _, subnet := range describeSubnetsResult.Subnets {
		for _, tag := range subnet.Tags {
			if tag.Key != nil && *tag.Key == "replicatedhq/public" {
				return *subnet.SubnetId, nil
			}
		}
	}

	subnetID, err := createSubnetInVPC(cfg, vpcID, "172.24.102.0/24", "us-west-1a", "replicatedhq/public")
	if err != nil {
		return "", errors.Wrap(err, "failed to create subnet")
	}

	return subnetID, nil
}

func ensurePrivateSubnetRouteTable(cfg aws.Config, vpcID string, subnetID string, natGatewayID string) error {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeRouteTablesResult, err := svc.DescribeRouteTables(ctx, describeRouteTablesInput)
	if err != nil {
		return errors.Wrap(err, "failed to describe route tables")
	}

	var routeTable *ec2types.RouteTable

FindRouteTable:
	for _, rt := range describeRouteTablesResult.RouteTables {
		for _, tag := range rt.Tags {
			if tag.Key != nil && *tag.Key == "replicatedhq/subnet-id" && tag.Value != nil && *tag.Value == subnetID {
				routeTable = &rt
				break FindRouteTable
			}
		}
	}

	if routeTable == nil {
		createRouteTableInput := &ec2.CreateRouteTableInput{
			VpcId: aws.String(vpcID),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeRouteTable,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("replicatedhq/kubectl-grid"),
							Value: aws.String("1"),
						},
						{
							Key:   aws.String("replicatedhq/subnet-id"),
							Value: aws.String(subnetID),
						},
					},
				},
			},
		}
		createRouteTableResult, err := svc.CreateRouteTable(ctx, createRouteTableInput)
		if err != nil {
			return errors.Wrap(err, "failed to create private route table")
		}

		routeTable = createRouteTableResult.RouteTable
	}

	associateRouteTableInput := &ec2.AssociateRouteTableInput{
		RouteTableId: routeTable.RouteTableId,
		SubnetId:     aws.String(subnetID),
	}
	_, err = svc.AssociateRouteTable(ctx, associateRouteTableInput)
	if err != nil {
		if !strings.Contains(err.Error(), "Resource.AlreadyAssociated") {
			return errors.Wrap(err, "failed to associate route table with subnet")
		}
	}

	// TODO: update route if it exists
	createRouteInput := &ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTableId,
		NatGatewayId:         aws.String(natGatewayID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
	}
	_, err = svc.CreateRoute(ctx, createRouteInput)
	if err != nil {
		if !strings.Contains(err.Error(), "RouteAlreadyExists") {
			return errors.Wrap(err, "failed to create private route")
		}
	}

	return nil
}

func ensurePublicSubnetRouteTable(cfg aws.Config, vpcID string, subnetID string, igwID string) error {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeRouteTablesResult, err := svc.DescribeRouteTables(ctx, describeRouteTablesInput)
	if err != nil {
		return errors.Wrap(err, "failed to describe route tables")
	}

	var routeTable *ec2types.RouteTable

FindRouteTable:
	for _, rt := range describeRouteTablesResult.RouteTables {
		for _, tag := range rt.Tags {
			if tag.Key != nil && *tag.Key == "replicatedhq/subnet-id" && tag.Value != nil && *tag.Value == subnetID {
				routeTable = &rt
				break FindRouteTable
			}
		}
	}

	if routeTable == nil {
		createRouteTableInput := &ec2.CreateRouteTableInput{
			VpcId: aws.String(vpcID),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeRouteTable,
					Tags: []ec2types.Tag{
						{
							Key:   aws.String("replicatedhq/kubectl-grid"),
							Value: aws.String("1"),
						},
						{
							Key:   aws.String("replicatedhq/subnet-id"),
							Value: aws.String(subnetID),
						},
					},
				},
			},
		}
		createRouteTableResult, err := svc.CreateRouteTable(ctx, createRouteTableInput)
		if err != nil {
			return errors.Wrap(err, "failed to create public route table")
		}

		routeTable = createRouteTableResult.RouteTable
	}

	associateRouteTableInput := &ec2.AssociateRouteTableInput{
		RouteTableId: routeTable.RouteTableId,
		SubnetId:     aws.String(subnetID),
	}
	_, err = svc.AssociateRouteTable(ctx, associateRouteTableInput)
	if err != nil {
		if !strings.Contains(err.Error(), "Resource.AlreadyAssociated") {
			return errors.Wrap(err, "failed to associate route table with subnet")
		}
	}

	createRouteInput := &ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTableId,
		GatewayId:            aws.String(igwID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
	}
	_, err = svc.CreateRoute(ctx, createRouteInput)
	if err != nil {
		return errors.Wrap(err, "failed to create public route")
	}

	return nil
}

func ensureElasticIP(cfg aws.Config) (string, error) {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	describeAddressesInput := &ec2.DescribeAddressesInput{
		Filters: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeAddressesResult, err := svc.DescribeAddresses(ctx, describeAddressesInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to describe addresses")
	}
	if len(describeAddressesResult.Addresses) > 0 {
		// this address may already be associated, but then it should be associated with our NAT gateway anyway
		return *describeAddressesResult.Addresses[0].AllocationId, nil
	}

	allocateAddressInput := &ec2.AllocateAddressInput{
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeElasticIp,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("replicatedhq/kubectl-grid"),
						Value: aws.String("1"),
					},
				},
			},
		},
	}

	allocateAddressResult, err := svc.AllocateAddress(ctx, allocateAddressInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to allocate address")
	}

	return *allocateAddressResult.AllocationId, nil
}

func ensureNATGateway(cfg aws.Config, subnetID string, allocationID string) (string, error) {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	describeNatGatewaysInput := &ec2.DescribeNatGatewaysInput{
		Filter: []ec2types.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []string{
					"replicatedhq/kubectl-grid",
				},
			},
		},
	}
	describeNatGatewaysResult, err := svc.DescribeNatGateways(ctx, describeNatGatewaysInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to describe nat gateways")
	}
	for _, gw := range describeNatGatewaysResult.NatGateways {
		if gw.State == ec2types.NatGatewayStatePending || gw.State == ec2types.NatGatewayStateAvailable {
			return *gw.NatGatewayId, nil
		}
	}

	createNatGatewayInput := &ec2.CreateNatGatewayInput{
		AllocationId: aws.String(allocationID),
		SubnetId:     aws.String(subnetID),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeNatgateway,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("replicatedhq/kubectl-grid"),
						Value: aws.String("1"),
					},
				},
			},
		},
	}

	createNatGatewayResult, err := svc.CreateNatGateway(ctx, createNatGatewayInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to create nat gateway")
	}

	gwID := *createNatGatewayResult.NatGateway.NatGatewayId

	if err := waitForNATGateway(cfg, gwID); err != nil {
		return "", errors.Wrap(err, "failed to wait for nat gateway")
	}

	return gwID, nil
}

func waitForNATGateway(cfg aws.Config, natGatewayID string) error {
	ctx := context.Background()
	svc := ec2.NewFromConfig(cfg)

	for i := 0; i < 10; i++ {
		describeNatGatewaysInput := &ec2.DescribeNatGatewaysInput{
			NatGatewayIds: []string{natGatewayID},
		}
		describeNatGatewaysResult, err := svc.DescribeNatGateways(ctx, describeNatGatewaysInput)
		if err != nil {
			return errors.Wrap(err, "failed to describe nat gateways")
		}
		for _, gw := range describeNatGatewaysResult.NatGateways {
			if gw.State == ec2types.NatGatewayStateAvailable {
				return nil
			}
		}

		time.Sleep(10 * time.Second)
	}

	return errors.New("timed out")
}

func createSubnetInVPC(cfg aws.Config, vpcID string, cidrBlock string, az string, tag string) (string, error) {
	svc := ec2.NewFromConfig(cfg)

	createSubnetInput := &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String(cidrBlock),
		AvailabilityZone: aws.String(az),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSubnet,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("replicatedhq/kubectl-grid"),
						Value: aws.String("1"),
					},
					{
						Key:   aws.String(tag),
						Value: aws.String("1"),
					},
				},
			},
		},
	}
	createSubnetResult, err := svc.CreateSubnet(context.Background(), createSubnetInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to create subnet")
	}

	return *createSubnetResult.Subnet.SubnetId, nil
}

func ensureEKSRoleARN(cfg aws.Config) (string, error) {
	svc := iam.NewFromConfig(cfg)

	listRolesInput := &iam.ListRolesInput{
		PathPrefix: aws.String("/replicatedhq/"),
	}

	listRolesResult, err := svc.ListRoles(context.Background(), listRolesInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to list roles")
	}
	if len(listRolesResult.Roles) > 0 {
		return *listRolesResult.Roles[0].Arn, nil
	}

	// empty inline policy
	rolePolicyJSON := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"Service": "eks.amazonaws.com",
				},
				"Action": "sts:AssumeRole",
			},
			{
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"Service": "ec2.amazonaws.com",
				},
				"Action": "sts:AssumeRole",
			},
		},
	}
	rolePolicy, err := json.Marshal(rolePolicyJSON)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal json")
	}

	createRoleInput := iam.CreateRoleInput{
		RoleName:                 aws.String("kubectl-grid"),
		Path:                     aws.String("/replicatedhq/"),
		AssumeRolePolicyDocument: aws.String(string(rolePolicy)),
	}
	result, err := svc.CreateRole(context.Background(), &createRoleInput)
	if err != nil {
		return "", errors.Wrap(err, "failed to create role")
	}

	if err := attachRolePolicy(cfg, "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"); err != nil {
		return "", errors.Wrap(err, "failed to attach policy 1")
	}
	if err := attachRolePolicy(cfg, "arn:aws:iam::aws:policy/AmazonEKSServicePolicy"); err != nil {
		return "", errors.Wrap(err, "failed to attach policy 2")
	}
	if err := attachRolePolicy(cfg, "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"); err != nil {
		return "", errors.Wrap(err, "failed to attach policy 3")
	}
	if err := attachRolePolicy(cfg, "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"); err != nil {
		return "", errors.Wrap(err, "failed to attach policy 4")
	}
	if err := attachRolePolicy(cfg, "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"); err != nil {
		return "", errors.Wrap(err, "failed to attach policy 5")
	}

	return *result.Role.Arn, nil
}

func attachRolePolicy(cfg aws.Config, policyName string) error {
	svc := iam.NewFromConfig(cfg)

	_, err := svc.AttachRolePolicy(context.Background(), &iam.AttachRolePolicyInput{
		PolicyArn: aws.String(policyName),
		RoleName:  aws.String("kubectl-grid"),
	})
	if err != nil {
		// what is happening here?
		if strings.Contains(err.Error(), "deserialization failed") {
			return nil
		}
		return errors.Wrap(err, "failed to attach policy")
	}

	return nil
}

func ensureEKSCluterControlPlane(cfg aws.Config, newEKSCluster *types.EKSNewClusterSpec, clusterName string, vpc *types.AWSVPC) (*ekstypes.Cluster, error) {
	svc := eks.NewFromConfig(cfg)

	version := newEKSCluster.Version
	if version == "" {
		version = "1.18"
	}

	input := &eks.CreateClusterInput{
		ClientRequestToken: aws.String(fmt.Sprintf("kubectl-grid-%x", md5.Sum([]byte(clusterName)))),
		Name:               aws.String(clusterName),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			SecurityGroupIds: vpc.SecurityGroupIDs,
			SubnetIds:        vpc.PrivateSubnetIDs,
		},
		RoleArn: aws.String(vpc.RoleArn),
		Version: aws.String(version),
	}

	createdCluster, err := svc.CreateCluster(context.Background(), input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create eks custer")
	}

	return createdCluster.Cluster, nil
}

func waitForClusterToBeActive(newEKSCluster *types.EKSNewClusterSpec, accessKeyID string, secretAccessKey string, clusterName string) error {
	resultCh := make(chan string)
	keepTrying := true
	go func() {
		for keepTrying {
			isReady, err := getEKSClusterIsReady(newEKSCluster.Region, accessKeyID, secretAccessKey, clusterName)
			if err != nil {
				resultCh <- fmt.Sprintf("error checking cluster status: %s", err.Error())
				return
			}

			if isReady {
				keepTrying = false
				resultCh <- ""
				return
			}

			time.Sleep(time.Second * 9)
		}
	}()

	select {
	case res := <-resultCh:
		if res != "" {
			return errors.New(res)
		}
		return nil
	case <-time.After(20 * time.Minute):
		keepTrying = false
		return errors.New("timeout waiting for cluster")
	}
}

func ensureEKSClusterNodeGroup(cfg aws.Config, cluster *ekstypes.Cluster, clusterName string, vpc *types.AWSVPC) (*ekstypes.Nodegroup, error) {
	svc := eks.NewFromConfig(cfg)

	nodeGroup, err := svc.CreateNodegroup(context.Background(), &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodeRole:      aws.String(vpc.RoleArn),
		NodegroupName: aws.String(clusterName),
		Subnets:       vpc.PrivateSubnetIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create eks node group")
	}

	return nodeGroup.Nodegroup, nil
}

func deleteEKSNodeGroup(cfg aws.Config, clusterName string, groupName string) error {
	svc := eks.NewFromConfig(cfg)

	deleteNodegroupInput := &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(groupName),
	}
	_, err := svc.DeleteNodegroup(context.Background(), deleteNodegroupInput)
	if err != nil && !isEKSNotFound(err) {
		return errors.Wrap(err, "failed to delete node group")
	}

	return nil
}

func waitEKSNodeGroupGone(cfg aws.Config, clusterName string, groupName string) error {
	svc := eks.NewFromConfig(cfg)

	for i := 0; i < 24; i++ {
		describeNodegroupInput := &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(groupName),
		}
		_, err := svc.DescribeNodegroup(context.Background(), describeNodegroupInput)
		if err != nil {
			if isEKSNotFound(err) {
				return nil
			}
			return errors.Wrap(err, "failed to describe node group")
		}

		time.Sleep(10 * time.Second)
	}

	return errors.New("timed out")
}

func deleteEKSCluster(cfg aws.Config, clusterName string) error {
	svc := eks.NewFromConfig(cfg)

	deleteClusterInput := &eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	}

	_, err := svc.DeleteCluster(context.Background(), deleteClusterInput)
	if err != nil {
		if isEKSNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "failed to delete cluster")
	}

	return nil
}
