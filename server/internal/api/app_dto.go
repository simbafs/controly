package api

import "github.com/simbafs/controly/server/internal/entity"

// CreateAppRequest for POST /api/app
type CreateAppRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdateAppRequest for PUT /api/app/:name
type UpdateAppRequest struct {
	Controls []map[string]any `json:"controls" binding:"required"`
}

// AppResponse is the response for app details
type AppResponse struct {
	Name     string           `json:"name"`
	Controls []map[string]any `json:"controls"`
}

func toAppResponse(app *entity.App) AppResponse {
	controls := make([]map[string]any, len(app.Controls()))
	for i, control := range app.Controls() {
		controls[i] = control.Map()
	}

	return AppResponse{
		Name:     app.Name(),
		Controls: controls,
	}
}
