package remotescoring

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/scheduler-plugins/apis/config"
)

const (
	Name          = "RemoteScoring"
	NodeScoresKey = Name + ".NodeScoresKey"
)

// RemoteScoring : scheduler plugin
type RemoteScoring struct {
	handle framework.Handle
	args   *config.RemoteScoringArgs
}

// NodeScoresStateData : computed at PreScore and used at Score
type NodeScoresStateData struct {
	scores framework.NodeScoreList
}

func (s *NodeScoresStateData) Clone() framework.StateData {
	return s
}

func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	klog.Infof("[RemoteScoring] Creating new instance of the RemoteScoring plugin")
	args, ok := obj.(*config.RemoteScoringArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type RemoteScoring, got %T", obj)
	}
	pl := &RemoteScoring{
		handle: handle,
		args:   args,
	}
	return pl, nil
}

func (pl *RemoteScoring) Name() string { return Name }

func (pl *RemoteScoring) PreScore(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodes []*v1.Node) *framework.Status {
	if !contains(pl.args.Namespaces, pod.Namespace) {
		klog.Infof("[RemoteScoring] Skip pod(%s) in namespace(%s)\n", pod.Name, pod.Namespace)
		return nil
	}

	nodeNames := make([]string, 0)
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	podLabels := make([]string, 0)
	for key, value := range pod.Labels {
		podLabels = append(podLabels, fmt.Sprintf("%s=%s", key, value))
	}

	scores, err := remoteCall(pl.args.Address, nodeNames, podLabels, pl.args.Namespaces)
	if err != nil {
		return nil
	}

	nodeScores := make(framework.NodeScoreList, 0)
	for i := 0; i < len(nodeNames); i++ {
		nodeScores = append(nodeScores, framework.NodeScore{Name: nodeNames[i], Score: int64(scores[i])})
	}

	nodeScoresStateData := &NodeScoresStateData{
		scores: nodeScores,
	}

	cycleState.Write(NodeScoresKey, nodeScoresStateData)
	return nil
}

// ScoreExtensions : an interface for Score extended functionality
func (pl *RemoteScoring) ScoreExtensions() framework.ScoreExtensions {
	return pl
}

func (pl *RemoteScoring) Score(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	if !contains(pl.args.Namespaces, pod.Namespace) {
		klog.Infof("[RemoteScoring] Skip pod(%s) in namespace(%s)\n", pod.Name, pod.Namespace)
		return 0, nil
	}

	nodeScores, err := getPreScoreState(cycleState)
	if err != nil {
		klog.Infof("[RemoteScoring] getPreScoreState: %s", err.Error())
		return 0, nil
	}
	for _, nodeScore := range nodeScores.scores {
		if nodeScore.Name == nodeName {
			klog.Infof("[RemoteScoring] Find node(%s) Score: %d\n", nodeScore.Name, nodeScore.Score)
			return nodeScore.Score, nil
		}
	}
	klog.Infof("[RemoteScoring] Fail to find node(%s) Score\n", nodeName)
	return 0, nil
}

func (pl *RemoteScoring) NormalizeScore(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var higherScore int64
	for _, node := range scores {
		if higherScore < node.Score {
			higherScore = node.Score
		}
	}

	if higherScore != 0 {
		for i, node := range scores {
			scores[i].Score = node.Score * framework.MaxNodeScore / higherScore
		}
	}

	klog.Infof("[RemoteScoring] Nodes final score: %v", scores)
	return nil
}

func getPreScoreState(cycleState *framework.CycleState) (*NodeScoresStateData, error) {
	nodeScoresStateData, err := cycleState.Read(NodeScoresKey)
	if err != nil {
		return nil, fmt.Errorf("reading %q from cycleState: %w", NodeScoresKey, err)
	}
	nodeScores, ok := nodeScoresStateData.(*NodeScoresStateData)
	if !ok {
		return nil, fmt.Errorf("invalid PreScore state, got type %T", nodeScoresStateData)
	}
	return nodeScores, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
