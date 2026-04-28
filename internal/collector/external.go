package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"xiaohongshu-tool/internal/storage"
)

type ExternalCommand struct {
	Command string
}

type Result struct {
	Items []storage.Item `json:"items"`
}

func (c ExternalCommand) Collect(ctx context.Context, target storage.Target) (Result, string, error) {
	parts := strings.Fields(c.Command)
	if len(parts) == 0 {
		return Result{}, "", fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("XHS_TARGET_ID=%d", target.ID),
		"XHS_TARGET_KIND="+target.Kind,
		"XHS_TARGET_NAME="+target.Name,
		"XHS_TARGET_URL="+target.URL,
		"XHS_TARGET_KEYWORD="+target.Keyword,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return Result{}, stderr.String(), err
	}
	var result Result
	if err := json.Unmarshal(out, &result); err != nil {
		return Result{}, stderr.String(), err
	}
	return result, stderr.String(), nil
}
