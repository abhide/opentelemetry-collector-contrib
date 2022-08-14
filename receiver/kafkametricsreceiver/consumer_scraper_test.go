// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkametricsreceiver

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestConsumerShutdown(t *testing.T) {
	client := newMockClient()
	client.closed = false
	client.close = nil
	client.Mock.
		On("Close").Return(nil).
		On("Closed").Return(false)
	scraper := consumerScraper{
		client: client,
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestConsumerShutdown_closed(t *testing.T) {
	client := newMockClient()
	client.closed = true
	client.Mock.
		On("Closed").Return(true)
	scraper := consumerScraper{
		client: client,
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestConsumerScraper_Name(t *testing.T) {
	s := consumerScraper{}
	assert.Equal(t, s.Name(), consumersScraperName)
}

func TestConsumerScraper_createConsumerScraper(t *testing.T) {
	sc := sarama.NewConfig()
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	cs, err := createConsumerScraper(context.Background(), Config{}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, cs)
}

func TestConsumerScraper_scrape_handles_client_error(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		return nil, fmt.Errorf("new client failed")
	}
	sc := sarama.NewConfig()
	cs, err := createConsumerScraper(context.Background(), Config{}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	_, err = cs.Scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handles_nil_client(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		return nil, fmt.Errorf("new client failed")
	}
	sc := sarama.NewConfig()
	cs, err := createConsumerScraper(context.Background(), Config{}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	err = cs.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestConsumerScraper_scrape_handles_clusterAdmin_error(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		client := newMockClient()
		client.Mock.
			On("Close").Return(nil)
		return client, nil
	}
	newClusterAdmin = func(addrs []string, conf *sarama.Config) (sarama.ClusterAdmin, error) {
		return nil, fmt.Errorf("new cluster admin failed")
	}
	sc := sarama.NewConfig()
	cs, err := createConsumerScraper(context.Background(), Config{}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	_, err = cs.Scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraperStart(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	sc := sarama.NewConfig()
	cs, err := createConsumerScraper(context.Background(), Config{}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	err = cs.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestConsumerScraper_createScraper_handles_invalid_topic_match(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	sc := sarama.NewConfig()
	cs, err := createConsumerScraper(context.Background(), Config{
		TopicMatch: "[",
	}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.Error(t, err)
	assert.Nil(t, cs)
}

func TestConsumerScraper_createScraper_handles_invalid_group_match(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	sc := sarama.NewConfig()
	cs, err := createConsumerScraper(context.Background(), Config{
		GroupMatch: "[",
	}, component.NewDefaultBuildInfo(), sc, zap.NewNop())
	assert.Error(t, err)
	assert.Nil(t, cs)
}

func TestConsumerScraper_scrape(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	config := createDefaultConfig().(*Config)
	cs := consumerScraper{
		client:       newMockClient(),
		logger:       zap.NewNop(),
		clusterAdmin: newMockClusterAdmin(),
		topicFilter:  filter,
		groupFilter:  filter,
		mb:           metadata.NewMetricsBuilder(config.Metrics, component.NewDefaultBuildInfo()),
	}
	md, err := cs.scrape(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, md)
}

func TestConsumerScraper_scrape_handlesListTopicError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	config := createDefaultConfig().(*Config)
	clusterAdmin := newMockClusterAdmin()
	client := newMockClient()
	clusterAdmin.topics = nil
	cs := consumerScraper{
		client:       client,
		logger:       zap.NewNop(),
		clusterAdmin: clusterAdmin,
		topicFilter:  filter,
		groupFilter:  filter,
		mb:           metadata.NewMetricsBuilder(config.Metrics, component.NewDefaultBuildInfo()),
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesListConsumerGroupError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	clusterAdmin.consumerGroups = nil
	cs := consumerScraper{
		client:       newMockClient(),
		logger:       zap.NewNop(),
		clusterAdmin: clusterAdmin,
		topicFilter:  filter,
		groupFilter:  filter,
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesDescribeConsumerError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	clusterAdmin.consumerGroupDescriptions = nil
	cs := consumerScraper{
		client:       newMockClient(),
		logger:       zap.NewNop(),
		clusterAdmin: clusterAdmin,
		topicFilter:  filter,
		groupFilter:  filter,
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesOffsetPartialError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	config := createDefaultConfig().(*Config)
	clusterAdmin := newMockClusterAdmin()
	client := newMockClient()
	client.offset = -1
	clusterAdmin.consumerGroupOffsets = nil
	cs := consumerScraper{
		client:       client,
		logger:       zap.NewNop(),
		groupFilter:  filter,
		topicFilter:  filter,
		clusterAdmin: clusterAdmin,
		mb:           metadata.NewMetricsBuilder(config.Metrics, component.NewDefaultBuildInfo()),
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesPartitionPartialError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	config := createDefaultConfig().(*Config)
	clusterAdmin := newMockClusterAdmin()
	client := newMockClient()
	client.partitions = nil
	clusterAdmin.consumerGroupOffsets = nil
	cs := consumerScraper{
		client:       client,
		logger:       zap.NewNop(),
		groupFilter:  filter,
		topicFilter:  filter,
		clusterAdmin: clusterAdmin,
		mb:           metadata.NewMetricsBuilder(config.Metrics, component.NewDefaultBuildInfo()),
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}
