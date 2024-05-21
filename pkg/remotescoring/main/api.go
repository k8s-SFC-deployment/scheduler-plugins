package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func main() {
	// 보낼 리스트 데이터입니다.
	nodeNames := []string{"ed-node2", "ed-node4"}
	podLabels := []string{"app=vnf", "hello=world"}
	address := "http://141.223.181.170:7000/step"

	scores, err := remoteCall(address, nodeNames, podLabels)

	if err != nil {
		fmt.Printf("Error: %s", err.Error())
		return
	}

	fmt.Println("Received items:")
	for _, score := range scores {
		fmt.Printf("Score: %d\n", score)
	}
}
