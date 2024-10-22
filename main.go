package main

import (
	"context"
	"flag"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"
)

var (
	baseLabels = []string{"dimension_DBInstanceIdentifier", "az", "secondary_az", "storage_type", "region", "db_instance_class", "engine"}
	tags       = []string{"purpose", "team", "region", "environment"}
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
	cacheTTL   time.Duration
	lastUpdate time.Time
	mu         sync.RWMutex
	updateMu   sync.Mutex
}

func createDynLabels(baseLabels []string, tags []string) []string {
	for _, tag := range tags {
		baseLabels = append(baseLabels, "tag_"+tag)
	}
	return baseLabels
}

func NewRDSExporter(sdkConfig aws.Config, ttl *time.Duration) *RDSExporter {
	return &RDSExporter{
		sdkConfig:  sdkConfig,
		cache:      []prometheus.Metric{},
		cacheTTL:   *ttl,
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
	e.mu.RLock()
	for _, metric := range e.cache {
		ch <- metric
	}
	cacheExpired := time.Since(e.lastUpdate) >= e.cacheTTL
	e.mu.RUnlock()

	if cacheExpired {
		go e.updateCache()
	}
}

func (e *RDSExporter) updateCache() {
	e.updateMu.Lock()
	defer e.updateMu.Unlock()

	accountClient := account.NewFromConfig(e.sdkConfig)
	regionOutput, err := accountClient.ListRegions(context.TODO(), &account.ListRegionsInput{
		RegionOptStatusContains: []types.RegionOptStatus{types.RegionOptStatusEnabled, types.RegionOptStatusEnabledByDefault}})
	if err != nil {
		log.Printf("Error listing regions: %v", err)
		return
	}

	var wg sync.WaitGroup
	metricsChan := make(chan prometheus.Metric, 100)

	for _, region := range regionOutput.Regions {
		wg.Add(1)
		go func(regionName string) {
			defer wg.Done()
			e.collectRegionMetrics(regionName, metricsChan)
		}(*region.RegionName)
	}
	go func() {
		wg.Wait()
		close(metricsChan)
	}()
	newCache := []prometheus.Metric{}
	for metric := range metricsChan {
		newCache = append(newCache, metric)
	}
	e.mu.Lock()
	e.cache = newCache
	e.lastUpdate = time.Now()
	e.mu.Unlock()
}

func (e *RDSExporter) collectRegionMetrics(regionName string, ch chan<- prometheus.Metric) {
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
			labels = append(labels, *instance.DBInstanceClass)
			labels = append(labels, *instance.Engine)

			// Build tags map
			tagMap := make(map[string]string)
			for _, tag := range tags {
				tagMap[tag] = ""
			}
			// fetch tags
			tagsOutput, err := rdsClient.ListTagsForResource(context.TODO(), &rds.ListTagsForResourceInput{
				ResourceName: instance.DBInstanceArn,
			})
			if err != nil {
				log.Printf("Error listing tags for RDS instance %s : %v", *instance.DBInstanceArn, err)
			} else {
				for _, tag := range tagsOutput.TagList {
					if tag.Key != nil && tag.Value != nil && slices.Contains(tags, *tag.Key) {
						tagMap[*tag.Key] = *tag.Value
					}
				}
			}

			// Add tags in correct order to labels
			for _, tag := range tags {
				labels = append(labels, tagMap[tag])
			}
			// New metrics
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

func main() {
	listenPort := flag.String("port", "6999", "Exporter listen port")
	cacheTTL := flag.Duration("cache_ttl", time.Hour, "Cache TTL")
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	flag.Parse()

	logger := log.WithFields(log.Fields{"app": "rds-exporter"})

	if err != nil {
		logger.Fatalf("unable to load SDK config: %v", err)
	}
	exporter := NewRDSExporter(sdkConfig, cacheTTL)
	prometheus.MustRegister(exporter)
	http.Handle("/metrics", promhttp.Handler())
	logger.Println("Listening on :" + *listenPort)
	http.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Write([]byte(`{"status":"OK"}`))
	})
	http.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Write([]byte(`{"status":"OK"}`))
	})
	if err := http.ListenAndServe(":"+*listenPort, nil); err != nil {
		logger.Fatalf("Error starting metric server: %s", err)
	}
}

//    TODO:
// Я так бегло глянул, чуть накидал комментов, чуть позже детальнее ещё гляну
// глянь пока на комменты ну и докинь ещё докерфайл плиз - по аналогии с остальными
// заодно можно и сборку прикрутить чтобы сразу всё было
// по архитектуре - что я бы ещё сделал
// я бы не стал городить логику с проверкой времени протухания кеша
// я бы ещё при запуске main запускал бы горутинку updateCache() по тику таймера ну или по forever-циклу со слипом - неважно сколько данные пролежали в кеше, главное что их нужно обновлять каждые n-тиков времени
// из метода коллект тогда можно всё вытащить, кроме отдачи метрик
// это кмк немного прозрачнее - те стартует приложенька ну и в фоне потихоньку апдейтит метрики
//
// ну и ещё допилить классику - сделать readyz / healthz эндпоинт, логи обвернуть в json
