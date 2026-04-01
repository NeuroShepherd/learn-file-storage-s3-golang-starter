package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 * 1024 * 1024 // 10 MB
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	imageData, imageHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't read thumbnail file", err)
		return
	}
	defer imageData.Close()

	mediaType, _, err := mime.ParseMediaType(imageHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		respondWithError(w, http.StatusBadRequest, "Invalid media type for thumbnail", nil)
		return
	}
	fileExtension := parts[1]
	fileName := videoID.String() + "." + fileExtension
	outputPath := filepath.Join(cfg.assetsRoot, fileName)

	// get video's metadata from the database
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video metadata", err)
		return
	}

	// check if the user is the owner of the video
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permission to upload a thumbnail for this video", nil)
		return
	}

	dst, err := os.Create(outputPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, imageData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy thumbnail file", err)
		return
	}

	dataUrl := "http://localhost:" + cfg.port + "/assets/" + fileName
	video.ThumbnailURL = &dataUrl

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video metadata with thumbnail URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
