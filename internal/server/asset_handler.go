package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) handleCORSPreflight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePublicAssetsOverview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	list, err := s.store.GetPublicAssets()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 1, "message": err.Error(), "data": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": list, "updated_at": store.NowShanghai()})
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := s.store.ListAssets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
}

func (s *Server) handleListAssetTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": modules.BuiltinAssetTemplates()})
}

func (s *Server) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var req store.Asset
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "资产 ID 与名称为必填项")
		return
	}
	existing, _ := s.store.GetAsset(req.ID)
	if existing != nil {
		writeError(w, http.StatusConflict, "资产 ID 已存在")
		return
	}
	if req.Category == "" {
		req.Category = "compute"
	}
	if req.WarnDays == "" {
		req.WarnDays = "[7, 3, 1]"
	}
	if req.ExtraConfig == "" {
		req.ExtraConfig = "{}"
	} else {
		var js map[string]any
		if json.Unmarshal([]byte(req.ExtraConfig), &js) != nil {
			writeError(w, http.StatusBadRequest, "额外配置需为合法 JSON")
			return
		}
	}
	req.LastCheckedAt = store.NowShanghai()
	if err := s.store.SaveAsset(req); err != nil {
		writeError(w, http.StatusInternalServerError, "创建资产失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetAsset(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "资产不存在")
		return
	}

	var req store.Asset
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	if req.Name != "" { existing.Name = req.Name }
	if req.Category != "" { existing.Category = req.Category }
	if req.Spec != "" { existing.Spec = req.Spec }
	if req.Price != "" { existing.Price = req.Price }
	if req.Status != "" { existing.Status = req.Status }
	if req.DueDate != "" { existing.DueDate = req.DueDate }
	existing.AutoRenew = req.AutoRenew
	if req.WarnDays != "" { existing.WarnDays = req.WarnDays }
	if req.ExtraConfig != "" {
		var js map[string]any
		if json.Unmarshal([]byte(req.ExtraConfig), &js) != nil {
			writeError(w, http.StatusBadRequest, "额外配置需为合法 JSON")
			return
		}
		existing.ExtraConfig = req.ExtraConfig
	}
	existing.LastCheckedAt = store.NowShanghai()

	if err := s.store.SaveAsset(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "保存资产失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少资产 ID")
		return
	}
	if err := s.store.DeleteAsset(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除资产失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
