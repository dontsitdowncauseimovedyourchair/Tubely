package main

import (
	rand2 "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
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

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, filename)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop saving upload", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
