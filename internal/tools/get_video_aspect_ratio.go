package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type ffprobeOut struct {
	Streams []Stream `json:"streams"`
}

type Stream struct {
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
}

func GetVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("flop running ffprobe command: %w", err)
	}

	var result ffprobeOut
	err = json.Unmarshal(output, &result)
	if err != nil {
		return "", fmt.Errorf("flop unmarshalling response: %w", err)
	}

	if len(result.Streams) == 0 {
		return "", fmt.Errorf("no command output")
	}

	ratio := result.Streams[0].DisplayAspectRatio
	if ratio == "16:9" {
		return "landscape", nil
	}
	if ratio == "9:16" {
		return "portrait", nil
	}
	return "other", nil
}
