package types

type AWSVPC struct {
	ID                string
	SecurityGroupIDs  []string
	PrivateSubnetIDs  []string
	PublicSubnetID    string
	InternetGatewayID string
	EIPAllocationID   string
	NATGatewayID      string
	RoleArn           string
}
