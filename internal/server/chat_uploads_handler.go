package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxChatImageUploadBytes = 8 * 1024 * 1024 // 8 MB

func detectAllowedImageExt(data []byte) (mimeType, ext string, ok bool) {
	if len(data) == 0 {
		return "", "", false
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/jpeg":
		return mime, ".jpg", true
	case "image/png":
		return mime, ".png", true
	case "image/webp":
		return mime, ".webp", true
	case "image/gif":
		return mime, ".gif", true
	default:
		return "", "", false
	}
}

func (s *Server) handleUploadChatImage(w http.ResponseWriter, r *http.Request) {
	var rawBytes []byte
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(ct, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxChatImageUploadBytes+512*1024)
		if err := r.ParseMultipartForm(maxChatImageUploadBytes + 256*1024); err != nil {
			writeError(w, http.StatusBadRequest, "图片上传超过 8MB 限制或表单格式无效")
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "未找到上传的图片文件字段 file")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxChatImageUploadBytes+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "读取图片失败")
			return
		}
		if len(data) > maxChatImageUploadBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "单张图片不能超过 8MB")
			return
		}
		rawBytes = data
	} else {
		var req struct {
			DataURL string `json:"data_url"`
		}
		if !decodeJSON(w, r, 12*1024*1024, &req) {
			return
		}
		rawURL := strings.TrimSpace(req.DataURL)
		commaIdx := strings.Index(rawURL, ",")
		if !strings.HasPrefix(rawURL, "data:image/") || commaIdx < 0 {
			writeError(w, http.StatusBadRequest, "仅支持有效的 data:image/... Base64 图片数据")
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawURL[commaIdx+1:]))
		if err != nil {
			writeError(w, http.StatusBadRequest, "Base64 图片解码失败")
			return
		}
		if len(decoded) > maxChatImageUploadBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "单张图片不能超过 8MB")
			return
		}
		rawBytes = decoded
	}

	mimeType, ext, ok := detectAllowedImageExt(rawBytes)
	if !ok {
		writeError(w, http.StatusBadRequest, "不支持的图片格式，请上传 JPG / PNG / WebP / GIF")
		return
	}

	dir := filepath.Join(s.store.DataDir(), "chat_uploads")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "创建图片存储目录失败")
		return
	}

	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	filename := hex.EncodeToString(buf) + ext
	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, rawBytes, 0o640); err != nil {
		writeError(w, http.StatusInternalServerError, "保存图片失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"url":      "/api/v2/chat/uploads/" + filename,
		"filename": filename,
		"mime":     mimeType,
		"size":     len(rawBytes),
	})
}

func isSafeUploadFilename(name string) bool {
	if name == "" || len(name) > 80 {
		return false
	}
	if filepath.Base(name) != name || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func (s *Server) handleServeChatUpload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if !isSafeUploadFilename(name) {
		writeError(w, http.StatusBadRequest, "无效图片文件名")
		return
	}
	fullPath := filepath.Join(s.store.DataDir(), "chat_uploads", name)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "图片不存在")
		return
	}
	mimeType, _, ok := detectAllowedImageExt(data)
	if !ok {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
