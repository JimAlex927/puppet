package task

import "encoding/json"

func DefaultPipelineJSON(name string) string {
	pipeline := map[string]any{
		"name":        name,
		"startNodeId": "hello",
		"agentSelector": map[string]any{
			"labels": []string{"local"},
		},
		"inputs": []map[string]any{},
		"nodes": []map[string]any{
			{
				"id":   "hello",
				"name": "输出 Hello",
				"type": "shell",
				"params": map[string]any{
					"workdir": "${workspace}",
					"script":  "echo hello pipeline",
				},
				"timeoutSeconds":  60,
				"retryTimes":      0,
				"nextNodeId":      "wait",
				"fallbackNodeId":  "",
				"continueOnError": false,
			},
			{
				"id":   "wait",
				"name": "等待 2 秒",
				"type": "sleep",
				"params": map[string]any{
					"seconds": 2,
				},
				"timeoutSeconds":  10,
				"retryTimes":      0,
				"nextNodeId":      "health",
				"fallbackNodeId":  "",
				"continueOnError": false,
			},
			{
				"id":   "health",
				"name": "HTTP 检查",
				"type": "http",
				"params": map[string]any{
					"method":  "GET",
					"url":     "https://example.com",
					"headers": map[string]any{},
				},
				"timeoutSeconds":  30,
				"retryTimes":      0,
				"nextNodeId":      "",
				"fallbackNodeId":  "",
				"continueOnError": false,
			},
		},
	}
	content, _ := json.MarshalIndent(pipeline, "", "  ")
	return string(content)
}
