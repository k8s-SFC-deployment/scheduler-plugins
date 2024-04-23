package netmarks

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/scheduler-plugins/apis/config"
)

type NetMarks struct {
	handle     framework.Handle
	prometheus *PrometheusHandle
}

const Name = "NetMarks"

var _ = framework.ScorePlugin(&NetMarks{})

func New(obj runtime.Object, h framework.Handle) (framework.Plugin, error) {
	args, ok := obj.(*config.NetMarksArgs)
	if !ok {
		return nil, fmt.Errorf("[NetMarks] want args to be of type NetMarksArgs, got %T", obj)
	}

	klog.Infof("[NetMarks] args received. TimeRangeInMinutes: %d, Address: %s", args.TimeRangeInMinutes, args.Address)

	return &NetMarks{
		handle:     h,
		prometheus: NewPrometheus(args.Address, time.Minute*time.Duration(args.TimeRangeInMinutes)),
	}, nil
}

func (n *NetMarks) Name() string {
	return Name
}

func (n *NetMarks) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	// Get dependent Services with target Pod
	dptSvcs, err := n.prometheus.GetDependentServicesQuery(context.TODO(), p.Labels["service.istio.io/canonical-name"])
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("error getting services depend on pod (%s): %s", p.Name, err))
	}
	for _, dstWorkloadName := range dptSvcs {
		fmt.Printf("[NetMarks] pod (%s) depend on service (%s)\n", p.Name, dstWorkloadName)
	}

	// Get Pod list in Node
	podsInNode, err := n.handle.ClientSet().CoreV1().Pods("testbed").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("error getting pods in node(%s): %s", nodeName, err))
	}

	var score int64
	for _, podInNode := range podsInNode.Items {
		fmt.Printf("[NetMarks] node(%s) have a pod(%s)\n", nodeName, podInNode.Name)
		for _, dstWorkloadName := range dptSvcs {
			podBandwidth, err := n.prometheus.GetPodBandwidthMeasure(context.TODO(), podInNode.Name, dstWorkloadName)
			if err != nil {
				klog.Fatal("[NetMarks] Error occur from podBandwidth call: %s", err.Error())
			}
			score += podBandwidth
		}
	}

	klog.Infof("[NetMarks] node '%s' bandwidth: %d", nodeName, score)
	return score, nil
}

func (n *NetMarks) ScoreExtensions() framework.ScoreExtensions {
	return n
}

func (n *NetMarks) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var higherScore int64
	for _, node := range scores {
		if higherScore < node.Score {
			higherScore = node.Score
		}
	}

	if higherScore == 0 {
		return nil
	}

	for i, node := range scores {
		scores[i].Score = framework.MaxNodeScore - (node.Score * framework.MaxNodeScore / higherScore)
	}

	klog.Infof("[NetMarks] Nodes final score: %v", scores)
	return nil
}
