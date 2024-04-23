package netmarks

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/klog/v2"
)

const (
	podMeasureQueryTemplate          = "sum by (pod) (sum_over_time(istio_request_bytes_sum{pod='%s', reporter='source', destination_workload='%s'}[%s]) + sum_over_time(istio_response_bytes_sum{pod='%s', reporter='source', destination_workload='%s'}[%s]))"
	sourceServicesQueryTemplate      = "sum by (source_workload) (istio_request_bytes_sum{pod=~'%s-.*', reporter='destination', destination_workload!='unknown'})"
	destinationServicesQueryTemplate = "sum by (destination_workload) (istio_request_bytes_sum{pod=~'%s-.*', reporter='source', destination_workload!='unknown'})"
)

type PrometheusHandle struct {
	api       v1.API
	address   string
	timeRange time.Duration
}

func NewPrometheus(address string, timeRange time.Duration) *PrometheusHandle {
	client, err := api.NewClient(api.Config{
		Address: address,
	})
	if err != nil {
		klog.Fatalf("[NetMarks] Error creating prometheus client: %s", err.Error())
	}
	return &PrometheusHandle{
		api:       v1.NewAPI(client),
		address:   address,
		timeRange: timeRange,
	}
}

func (p *PrometheusHandle) GetPodBandwidthMeasure(ctx context.Context, podName string, dstWorkloadName string) (int64, error) {
	query := getPodBandwidthQuery(podName, dstWorkloadName, p.timeRange)
	res, err := p.query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("[NetMarks] Error querying prometheus: %w", err)
	}

	podMeasure := res.(model.Vector)
	if len(podMeasure) == 0 {
		return 0, nil
	} else if len(podMeasure) != 1 {
		return 0, fmt.Errorf("[NetMarks] Invalid response, expected 1 value, got %d", len(podMeasure))
	}

	return int64(podMeasure[0].Value), nil
}

func (p *PrometheusHandle) GetDestinationServices(ctx context.Context, sourceSvcName string) ([]string, error) {
	query := getDestinationServicesQuery(sourceSvcName)
	res, err := p.query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("[NetMarks] Error querying prometheus: %w", err)
	}

	services := make([]string, 0)
	measures := res.(model.Vector)
	for _, measure := range measures {
		metric := measure.Metric.String()

		services = append(services, metric[23:len(metric)-2])
	}
	return services, nil
}

func (p *PrometheusHandle) GetSourceServicesQuery(ctx context.Context, destinationSvcName string) ([]string, error) {
	query := getSourceServicesQuery(destinationSvcName)
	res, err := p.query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("[NetMarks] Error querying prometheus: %w", err)
	}

	services := make([]string, 0)
	measures := res.(model.Vector)
	for _, measure := range measures {
		metric := measure.Metric.String()
		services = append(services, metric[23:len(metric)-2])
	}
	return services, nil
}

func (p *PrometheusHandle) GetDependentServicesQuery(ctx context.Context, svcName string) ([]string, error) {
	srcSvcs, err := p.GetSourceServicesQuery(ctx, svcName)
	if err != nil {
		return nil, err
	}
	dstSvcs, err := p.GetDestinationServices(ctx, svcName)
	if err != nil {
		return nil, err
	}
	return append(srcSvcs, dstSvcs...), nil
}

func getPodBandwidthQuery(podName string, dstWorkloadName string, timeRange time.Duration) string {
	return fmt.Sprintf(
		podMeasureQueryTemplate,
		podName, dstWorkloadName, timeRange,
		podName, dstWorkloadName, timeRange,
	)
}

func getDestinationServicesQuery(sourceSvcName string) string {
	return fmt.Sprintf(destinationServicesQueryTemplate, sourceSvcName)
}

func getSourceServicesQuery(destinationSvcName string) string {
	return fmt.Sprintf(sourceServicesQueryTemplate, destinationSvcName)
}

func (p *PrometheusHandle) query(ctx context.Context, query string) (model.Value, error) {
	results, warnings, err := p.api.Query(ctx, query, time.Now())

	if len(warnings) > 0 {
		klog.Warningf("[NetMarks] Warning: %v\n", warnings)
	}
	return results, err
}
