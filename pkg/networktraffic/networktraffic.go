package networktraffic

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/scheduler-plugins/apis/config"
)

type NetworkTraffic struct {
	handle     framework.Handle
	prometheus *PrometheusHandle
}

const Name = "NetworkTraffic"

var _ = framework.ScorePlugin(&NetworkTraffic{})

func New(obj runtime.Object, h framework.Handle) (framework.Plugin, error) {
	args, ok := obj.(*config.NetworkTrafficArgs)
	if !ok {
		return nil, fmt.Errorf("[NetworkTraffic] want args to be of type NetworkTrafficArgs, got %T", obj)
	}

	klog.Infof("[NetworkTraffic] args received. NetworkInterface: %s; TimeRangeInMinutes: %d, Address: %s", args.NetworkInterface, args.TimeRangeInMinutes, args.Address)

	return &NetworkTraffic{
		handle:     h,
		prometheus: NewPrometheus(args.Address, args.NetworkInterface, time.Minute*time.Duration(args.TimeRangeInMinutes)),
	}, nil
}

func (n *NetworkTraffic) Name() string {
	return Name
}

func (n *NetworkTraffic) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	nodeBandwidth, err := n.prometheus.GetNodeBandwidthMeasure(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("error getting node bandwidth measure: %s", err))
	}

	klog.Infof("[NetworkTraffic] node '%s' bandwidth: %s", nodeName, nodeBandwidth.Value)
	return int64(nodeBandwidth.Value), nil
}

func (n *NetworkTraffic) ScoreExtensions() framework.ScoreExtensions {
	return n
}

func (n *NetworkTraffic) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var higherScore int64
	for _, node := range scores {
		if higherScore < node.Score {
			higherScore = node.Score
		}
	}

	for i, node := range scores {
		scores[i].Score = framework.MaxNodeScore - (node.Score * framework.MaxNodeScore / higherScore)
	}

	klog.Infof("[NetworkTraffic] Nodes final score: %v", scores)
	return nil
}
