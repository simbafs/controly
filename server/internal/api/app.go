
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/simbafs/controly/server/internal/entity"
	"github.com/simbafs/controly/server/internal/repository"
)

type AppAPI struct {
	appRepo repository.App
}

func NewAppAPI(appRepo repository.App) *AppAPI {
	return &AppAPI{
		appRepo: appRepo,
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
	var app entity.App
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.appRepo.Put(c.Request.Context(), app.Name(), &app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, app)
}

func (a *AppAPI) Update(c *gin.Context) {
	name := c.Param("name")
	var app entity.App
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.appRepo.Put(c.Request.Context(), name, &app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, app)
}

func (a *AppAPI) List(c *gin.Context) {
	apps, err := a.appRepo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, apps)
}

func (a *AppAPI) Get(c *gin.Context) {
	name := c.Param("name")
	app, err := a.appRepo.Get(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, app)
}

func (a *AppAPI) Delete(c *gin.Context) {
	name := c.Param("name")
	if err := a.appRepo.Delete(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
