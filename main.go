package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"
)

var (
	baseLabels = []string{"dimension_DBInstanceIdentifier", "az", "secondary_az", "storage_type", "region", "name", "db_instance_class", "engine"}
	tags       = []string{"billing", "purpose", "team", "region", "environment"}
	dynLabels  = createDynLabels(baseLabels, tags)

	allocatedStorageDesc = prometheus.NewDesc(
		"aws_rds_allocated_storage",
		"Allocated storage for RDS instance in GB",
		dynLabels, nil,
	)
	maxAllocatedStorageDesc = prometheus.NewDesc(
		"aws_rds_max_allocated_storage",
		"Max allocated storage for RDS instance in GB",
		dynLabels, nil,
	)
	iopsDesc = prometheus.NewDesc(
		"aws_rds_iops",
		"IOPS for RDS instance",
		dynLabels, nil,
	)
	storageThroughputDesc = prometheus.NewDesc(
		"aws_rds_storage_throughput",
		"Storage throughput for RDS instance",
		dynLabels, nil,
	)
)

type RDSExporter struct {
	sdkConfig  aws.Config
	cache      []prometheus.Metric
	lastUpdate time.Time
	mu         sync.Mutex
}

func createDynLabels(baseLabels []string, tags []string) []string {
	for _, tag := range tags {
		baseLabels = append(baseLabels, "tag_"+tag)
	}
	return baseLabels
}

func NewRDSExporter(sdkConfig aws.Config) *RDSExporter {
	return &RDSExporter{
		sdkConfig:  sdkConfig,
		cache:      []prometheus.Metric{},
		lastUpdate: time.Time{},
	}
}

func (e *RDSExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- allocatedStorageDesc
	ch <- maxAllocatedStorageDesc
	ch <- iopsDesc
	ch <- storageThroughputDesc
}

func (e *RDSExporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if time.Since(e.lastUpdate) > time.Hour {
		for _, metric := range e.cache {
			ch <- metric
		}
		return
	}

	// Update cache
	e.cache = []prometheus.Metric{}
	accountClient := account.NewFromConfig(e.sdkConfig)
	regionOutput, err := accountClient.ListRegions(context.TODO(), &account.ListRegionsInput{
		RegionOptStatusContains: []types.RegionOptStatus{types.RegionOptStatusEnabled, types.RegionOptStatusEnabledByDefault}})
	if err != nil {
		log.Printf("Error listing regions: %v", err)
		return
	}

	for _, region := range regionOutput.Regions {
		regionName := *region.RegionName
		rdsClient := rds.NewFromConfig(e.sdkConfig, func(o *rds.Options) { o.Region = regionName })
		var marker *string
		for {
			output, err := rdsClient.DescribeDBInstances(context.TODO(), &rds.DescribeDBInstancesInput{Marker: marker})
			if err != nil {
				log.Printf("Couldn't list RDS instances in region %s : %v", regionName, err)
				break
			}
			//{"dimension_DBInstanceIdentifier", "az", "secondary_az", "storage_type", "region", "name", "db_instance_class", "engine"}
			for _, instance := range output.DBInstances {
				labels := []string{}
				labels = append(labels, *instance.DBInstanceIdentifier)
				labels = append(labels, *instance.AvailabilityZone)
				if instance.SecondaryAvailabilityZone != nil {
					labels = append(labels, *instance.SecondaryAvailabilityZone)
				} else {
					labels = append(labels, "")
				}
				labels = append(labels, *instance.StorageType)
				labels = append(labels, regionName)
				labels = append(labels, *instance.DBInstanceArn)
				labels = append(labels, *instance.DBInstanceClass)
				labels = append(labels, *instance.Engine)

				// fetch tags
				tagsOutput, err := rdsClient.ListTagsForResource(context.TODO(), &rds.ListTagsForResourceInput{
					ResourceName: instance.DBInstanceArn,
				})
				if err != nil {
					log.Printf("Error listing tags for RDS instance %s : %v", *instance.DBInstanceArn, err)
				} else {
					if len(tagsOutput.TagList) == 0 {
						for range tags {
							labels = append(labels, "")
						}
					} else {
						for _, tag := range tagsOutput.TagList {
							if tag.Key != nil && tag.Value != nil && slices.Contains(tags, *tag.Key) {
								labels = append(labels, *tag.Value)
							}
						}
					}
				}
				if instance.AllocatedStorage != nil {
					metric := prometheus.MustNewConstMetric(allocatedStorageDesc, prometheus.GaugeValue, float64(*instance.AllocatedStorage), labels...)
					e.cache = append(e.cache, metric)
					ch <- metric
				}
				if instance.MaxAllocatedStorage != nil {
					metric := prometheus.MustNewConstMetric(maxAllocatedStorageDesc, prometheus.GaugeValue, float64(*instance.MaxAllocatedStorage), labels...)
					e.cache = append(e.cache, metric)
					ch <- metric
				}
				if instance.Iops != nil {
					metric := prometheus.MustNewConstMetric(iopsDesc, prometheus.GaugeValue, float64(*instance.Iops), labels...)
					e.cache = append(e.cache, metric)
					ch <- metric
				}
				if instance.StorageThroughput != nil {
					metric := prometheus.MustNewConstMetric(storageThroughputDesc, prometheus.GaugeValue, float64(*instance.StorageThroughput), labels...)
					e.cache = append(e.cache, metric)
					ch <- metric
				}
			}

			if output.Marker == nil {
				break
			} else {
				marker = output.Marker
			}
		}
	}
	e.lastUpdate = time.Now()
}

func main() {
	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	exporter := NewRDSExporter(sdkConfig)
	prometheus.MustRegister(exporter)
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
