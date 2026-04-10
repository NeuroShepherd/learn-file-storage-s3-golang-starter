package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var newBuffer bytes.Buffer
	cmd.Stdout = &newBuffer
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	// unmarhal the output to get the width and height of the video stream
	type FFProbeOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	var output FFProbeOutput
	err = json.Unmarshal(newBuffer.Bytes(), &output)
	if err != nil {
		return "", err
	}

	if len(output.Streams) == 0 {
		return "", fmt.Errorf("no video streams found")
	}

	width := output.Streams[0].Width
	height := output.Streams[0].Height

	if width == 0 || height == 0 {
		return "", fmt.Errorf("invalid video dimensions")
	}

	aspectRatio := float64(width) / float64(height)

	if math.Abs(aspectRatio-16.0/9.0) < 0.01 {
		return "16:9", nil
	} else if math.Abs(aspectRatio-9.0/16.0) < 0.01 {
		return "9:16", nil
	}

	return "other", nil

}
