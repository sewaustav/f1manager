package http

import (
	"f1/internal/web/dto"
	
	"github.com/gin-gonic/gin"
)

func (h *HttpHandler) RegisterGroup(c *gin.Context) {
	ctx := c.Request.Context()
	
	var group dto.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "unauthorized"})
		return
	}
	
	if err := h.userData.RegisterGroup(ctx, user, group); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{"message": "group registered"})
	
}

func (h *HttpHandler) JoinGroup(c *gin.Context) {
	ctx := c.Request.Context()
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "unauthorized"})
		return
	}
	
	var group dto.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.userData.JoinGroup(ctx, user, group); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "group joined"})

}
