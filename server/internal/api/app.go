package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/simbafs/controly/server/internal/entity"
	"github.com/simbafs/controly/server/internal/usecase"
)

type CreateAppBody struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AppBody struct {
	Name    string           `json:"name"`
	Control []map[string]any `json:"controls"`
}

type AppAPI struct {
	appUsecase usecase.App
}

func NewAppAPI(appUsecase usecase.App) *AppAPI {
	return &AppAPI{
		appUsecase: appUsecase,
	}
}

func (a *AppAPI) Setup(r *gin.Engine) {
	app := r.Group("/api/app")
	app.POST("", a.Create)
	app.PUT("/:name", a.Update)
	app.GET("", a.List)
	app.GET("/:name", a.Get)
	app.DELETE("/:name", a.Delete)
}

func (a *AppAPI) Create(c *gin.Context) {
	var body CreateAppBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("create app", "app", body)

	app, err := a.appUsecase.CreateApp(c.Request.Context(), body.Name, body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, buildAppResp(app))
}

func (a *AppAPI) Update(c *gin.Context) {
	name := c.Param("name")
	var body AppBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app, err := a.appUsecase.UpdateApp(c.Request.Context(), name, body.Control)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, buildAppResp(app))
}

func buildAppResp(app *entity.App) AppBody {
	controls := make([]map[string]any, len(app.Controls()))
	for i, control := range app.Controls() {
		controls[i] = control.Map()
	}

	return AppBody{
		Name:    app.Name(),
		Control: controls,
	}
}

func (a *AppAPI) List(c *gin.Context) {
	apps, err := a.appUsecase.ListApps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	body := make([]AppBody, len(apps))
	for i, app := range apps {
		body[i] = buildAppResp(app)
	}

	c.JSON(http.StatusOK, body)
}

func (a *AppAPI) Get(c *gin.Context) {
	name := c.Param("name")
	app, err := a.appUsecase.GetApp(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, buildAppResp(app))
}

func (a *AppAPI) Delete(c *gin.Context) {
	name := c.Param("name")
	if err := a.appUsecase.DeleteApp(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
