package main

import (
	rand2 "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/tools"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	reader := http.MaxBytesReader(w, r.Body, 1<<30)
	r.Body = reader

	rawVideoID := r.PathValue("videoID")
	videoID, err := uuid.Parse(rawVideoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "flop video not found", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "flop video not found", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "flop auth token", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "flop auth token", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "not your account", err)
		return
	}

	multipartVid, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "flop getting mutliform vid", err)
		return
	}
	defer multipartVid.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "flop media-type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusForbidden, "media not a video", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, multipartVid)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}

	ratio, err := tools.GetVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}

	newPath, err := tools.ProcessVideoForFastStreaming(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}

	toUpload, err := os.Open(newPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}
	defer toUpload.Close()
	defer os.Remove(toUpload.Name())

	randstuff := make([]byte, 32)
	_, err = rand2.Read(randstuff)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading", err)
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(randstuff)
	filename := ratio + "/" + encoded + ".mp4"

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        toUpload,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop uploading to the big leagues", err)
		return
	}

	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, filename)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop saving upload", err)
		return
	}

	video, err = cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop generating url", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	components := strings.SplitN(*video.VideoURL, ",", 2)
	if len(components) < 2 {
		return video, fmt.Errorf("flop videoURL")
	}
	bucket := components[0]
	key := components[1]
	presignedURL, err := tools.GeneratePresignedURL(cfg.s3Client, bucket, key, 30*time.Minute)
	if err != nil {
		return video, fmt.Errorf("flop generating presigned url: %w", err)
	}
	video.VideoURL = &presignedURL
	return video, nil
}
