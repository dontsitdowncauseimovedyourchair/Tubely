package tools

import (
	"fmt"
	"os/exec"
)

type ffmpegOut struct {
}

func ProcessVideoForFastStreaming(filepath string) (string, error) {
	outPath := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPath)
	_, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("flop running ffmpeg command: %w", err)
	}
	return outPath, nil
}
