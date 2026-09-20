package main

import (
	rand2 "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

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

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "flop parsing form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	media_type := header.Header.Get("Content-Type")

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "flop video, not found", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "flop auth", err)
		return
	}

	mt, _, err := mime.ParseMediaType(media_type)
	if err != nil || (mt != "image/jpeg" && mt != "image/png") {
		respondWithError(w, http.StatusBadRequest, "unsupported file format", err)
		return
	}

	extensions, err := mime.ExtensionsByType(mt)
	if err != nil || len(extensions) == 0 {
		respondWithError(w, http.StatusBadRequest, "flop content-type", err)
		return
	}
	extension := extensions[0]

	randstuff := make([]byte, 32)
	_, err = rand2.Read(randstuff)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop making file", err)
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(randstuff)

	tnPath := filepath.Join(cfg.assetsRoot, encoded+extension)
	assetFile, err := os.Create(tnPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop creating path", err)
		return
	}
	defer assetFile.Close()

	_, err = io.Copy(assetFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop copying data", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, encoded+extension)
	video.ThumbnailURL = &url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "flop updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
