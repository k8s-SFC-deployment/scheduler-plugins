package remotescoring

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type Payload struct {
	NodeNames []string `json:"nodeNames"`
	PodLabels []string `json:"podLabels"`
}

func remoteCall(address string, nodeNames []string, podLabels []string) ([]int, error) {
	payload := Payload{
		NodeNames: nodeNames,
		PodLabels: podLabels,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(address, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var scores []int
	if err := json.Unmarshal(body, &scores); err != nil {
		return nil, err
	}

	return scores, nil
}
