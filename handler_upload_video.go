package main

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// check byte limit
	const maxVideoSizeBytes = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxVideoSizeBytes)

	// extract video id
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// authenticate user
	// get the JWT from the Authorization header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	// validate the JWT and extract the user ID
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// get video metadata from database
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	// check that the user is the owner of the video
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permission to upload a video for this video ID", nil)
		return
	}

	videoFile, videoFileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't read video file", err)
		return
	}
	defer videoFile.Close()

	// validate the video file type i.e. mp4
	mediaType, _, err := mime.ParseMediaType(videoFileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}

	// save the video to a temporary file
	tempFile, err := os.CreateTemp("", "upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temporary file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save video file", err)
		return
	}

	// get aspect ratio
	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
		return
	}

	// determine video aspect mode based on aspect ratio
	videoAspectMode := ""
	switch aspectRatio {
	case "16:9":
		videoAspectMode = "landscape"
	case "9:16":
		videoAspectMode = "portrait"
	default:
		videoAspectMode = "other"
	}

	tempFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process video for fast start", err)
		return
	}
	defer os.Remove(tempFilePath)

	// open the processed video filefor reading
	tempFile, err = os.Open(tempFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open processed video file", err)
		return
	}
	defer tempFile.Close()

	// reset the file pointer to the beginning of the file
	tempFile.Seek(0, io.SeekStart)

	// use 32 random bytes for the file name to avoid collisions and keep the original file extension
	fileName := randomHexString(32) + ".mp4"

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(videoAspectMode + "/" + fileName),
		Body:        tempFile,
		ContentType: aws.String(mediaType),
	},
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video to S3", err)
		return
	}

	videoURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + videoAspectMode + "/" + fileName
	video.VideoURL = &videoURL
	video.UpdatedAt = time.Now()

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)

}

func randomHexString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate random bytes")
	}
	return hex.EncodeToString(bytes)
}
