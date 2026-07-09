package deploy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

type ScriptOutput struct {
	Log           string                   `json:"log"`
	Progress      []map[string]interface{} `json:"progress"`
	Result        map[string]interface{}   `json:"result,omitempty"`
	GeneratedNode map[string]interface{}   `json:"generated_node,omitempty"`
}

func ParseScriptOutput(output string) (*ScriptOutput, error) {
	parsed := &ScriptOutput{}
	var log strings.Builder
	var node strings.Builder
	inNode := false

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "SBM_NODE_BEGIN":
			inNode = true
			node.Reset()
		case line == "SBM_NODE_END":
			inNode = false
			var generated map[string]interface{}
			if err := json.Unmarshal([]byte(node.String()), &generated); err != nil {
				return nil, fmt.Errorf("parse SBM node JSON: %w", err)
			}
			parsed.GeneratedNode = generated
		case inNode:
			node.WriteString(line)
			node.WriteByte('\n')
		case strings.HasPrefix(line, "SBM_PROGRESS "):
			progress, err := parseMarkerJSON(line, "SBM_PROGRESS ")
			if err != nil {
				return nil, err
			}
			parsed.Progress = append(parsed.Progress, progress)
		case strings.HasPrefix(line, "SBM_RESULT "):
			result, err := parseMarkerJSON(line, "SBM_RESULT ")
			if err != nil {
				return nil, err
			}
			parsed.Result = result
		default:
			log.WriteString(line)
			log.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if inNode {
		return nil, fmt.Errorf("missing SBM_NODE_END")
	}
	parsed.Log = log.String()
	return parsed, nil
}

func parseMarkerJSON(line, prefix string) (map[string]interface{}, error) {
	var decoded map[string]interface{}
	raw := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("parse %s JSON: %w", strings.TrimSpace(prefix), err)
	}
	return decoded, nil
}
