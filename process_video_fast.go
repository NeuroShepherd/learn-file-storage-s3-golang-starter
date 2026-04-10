package main

import "os/exec"

func processVideoForFastStart(filePath string) (string, error) {
	tempFilePath := filePath + ".processing"

	exc := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", tempFilePath)
	err := exc.Run()
	if err != nil {
		return "", err
	}
	return tempFilePath, nil
}
