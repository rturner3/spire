package aws

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	defaultNewClientCallback = newClient
)

type Client interface {
	ec2.DescribeInstancesAPIClient
	iam.GetInstanceProfileAPIClient
}

type clientsCache struct {
	mu        sync.RWMutex
	config    *SessionConfig
	clients   map[string]Client
	newClient newClientCallback
}

type newClientCallback func(config *SessionConfig, region string, asssumeRoleARN string) (Client, error)

func newClientsCache(newClient newClientCallback) *clientsCache {
	return &clientsCache{
		clients:   make(map[string]Client),
		newClient: newClient,
	}
}

func (cc *clientsCache) configure(config SessionConfig) {
	cc.mu.Lock()
	cc.clients = make(map[string]Client)
	cc.config = &config
	cc.mu.Unlock()
}

func (cc *clientsCache) getClient(region, accountID string) (Client, error) {
	// do an initial check to see if p client for this region already exists
	cacheKey := accountID + "@" + region

	cc.mu.RLock()
	client, ok := cc.clients[region]
	cc.mu.RUnlock()
	if ok {
		return client, nil
	}

	// no client for this region. make one.
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// more than one thread could be racing to create p client (since we had
	// to drop the read lock to take the write lock), so double check somebody
	// hasn't beat us to it.
	client, ok = cc.clients[cacheKey]
	if ok {
		return client, nil
	}

	if cc.config == nil {
		return nil, status.Error(codes.FailedPrecondition, "not configured")
	}

	var asssumeRoleArn string
	if cc.config.AssumeRole != "" {
		asssumeRoleArn = fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, cc.config.AssumeRole)
	}

	client, err := cc.newClient(cc.config, region, asssumeRoleArn)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client: %v", err)
	}

	cc.clients[cacheKey] = client
	return client, nil
}

func newClient(config *SessionConfig, region string, asssumeRoleARN string) (Client, error) {
	conf, err := newAWSConfig(config.AccessKeyID, config.SecretAccessKey, region, asssumeRoleARN)
	if err != nil {
		return nil, err
	}
	return struct {
		iam.GetInstanceProfileAPIClient
		ec2.DescribeInstancesAPIClient
	}{
		GetInstanceProfileAPIClient: iam.NewFromConfig(conf),
		DescribeInstancesAPIClient:  ec2.NewFromConfig(conf),
	}, nil
}
