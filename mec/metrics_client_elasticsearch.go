package mec

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v7"
)

const (
	// esHostNameField is the field name of host name in the document
	esHostNameField = "host.hostname"
	// esCpuUsageField is the field name of cpu usage in the document
	esCpuUsageField = "host.cpu.usage"
	// esMemUsageField is the field name of mem usage in the document
	esMemUsageField = "system.memory.actual.used.pct"
)

type ElasticsearchMetricsClient struct {
	address   string
	indexName string
	es        *elasticsearch.Client
}

func NewElasticsearchMetricsClient(address string, conf map[string]string) (*ElasticsearchMetricsClient, error) {
	e := &ElasticsearchMetricsClient{}
	indexConf := conf["elasticsearch.index"]
	if len(indexConf) == 0 {
		e.indexName = "metricbeat-*"
	} else {
		e.indexName = indexConf
	}
	var err error
	e.es, err = elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{address},
		Username:  conf["elasticsearch.username"],
		Password:  conf["elasticsearch.password"],
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ElasticsearchMetricsClient) NodeMetricsAvg(ctx context.Context, nodeName string, period string) (*NodeMetrics, error) {
	nodeMetrics := &NodeMetrics{}
	var buf bytes.Buffer
	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"range": map[string]interface{}{
							"@timestamp": map[string]interface{}{
								"gte": "now-" + period,
								"lt":  "now",
							},
						},
					},
					{
						"term": map[string]interface{}{
							esHostNameField: nodeName,
						},
					},
				},
			},
		},
		"aggs": map[string]interface{}{
			"cpu": map[string]interface{}{
				"avg": map[string]interface{}{
					"field": esCpuUsageField,
				},
			},
			"mem": map[string]interface{}{
				"avg": map[string]interface{}{
					"field": esMemUsageField,
				},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}
	res, err := e.es.Search(
		e.es.Search.WithContext(ctx),
		e.es.Search.WithIndex(e.indexName),
		e.es.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}
	aggs := r["aggregations"].(map[string]interface{})
	cpuUsage := aggs["cpu"].(map[string]interface{})["value"].(float64)
	memUsage := aggs["mem"].(map[string]interface{})["value"].(float64)
	nodeMetrics.Cpu = cpuUsage
	nodeMetrics.Memory = memUsage
	return nodeMetrics, nil
}
