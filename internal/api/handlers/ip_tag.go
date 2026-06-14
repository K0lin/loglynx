// MIT License
//
// # Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package handlers

import (
	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// IPTagHandler handles IP tagging requests
type IPTagHandler struct {
	ipTagRepo repositories.IPTagRepository
	logger    *pterm.Logger
}

// NewIPTagHandler creates a new IP tag handler
func NewIPTagHandler(ipTagRepo repositories.IPTagRepository, logger *pterm.Logger) *IPTagHandler {
	return &IPTagHandler{
		ipTagRepo: ipTagRepo,
		logger:    logger,
	}
}

// CreateOrUpdateTag creates or updates an IP tag
func (h *IPTagHandler) CreateOrUpdateTag(c *gin.Context) {
	var request struct {
		IPAddress    string `json:"ip_address" binding:"required"`
		FriendlyName string `json:"friendly_name"`
		Tags         string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.ipTagRepo.FindByIP(request.IPAddress)
	if err != nil {
		// Create new tag
		tag = &models.IPTag{
			IPAddress:    request.IPAddress,
			FriendlyName: request.FriendlyName,
			Tags:         request.Tags,
		}
		err = h.ipTagRepo.Create(tag)
	} else {
		// Update existing tag
		tag.FriendlyName = request.FriendlyName
		tag.Tags = request.Tags
		err = h.ipTagRepo.Update(tag)
	}

	if err != nil {
		h.logger.WithCaller().Error("Failed to save IP tag", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save IP tag"})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// DeleteTag deletes an IP tag
func (h *IPTagHandler) DeleteTag(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address required"})
		return
	}

	err := h.ipTagRepo.Delete(ip)
	if err != nil {
		h.logger.WithCaller().Error("Failed to delete IP tag", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete IP tag"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted"})
}

// GetTag gets an IP tag by IP
func (h *IPTagHandler) GetTag(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address required"})
		return
	}

	tag, err := h.ipTagRepo.FindByIP(ip)
	if err != nil {
		// Return empty tag for new IPs
		c.JSON(http.StatusOK, models.IPTag{IPAddress: ip})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// GetAllTags gets all IP tags
func (h *IPTagHandler) GetAllTags(c *gin.Context) {
	tags, err := h.ipTagRepo.FindAll()
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP tags", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP tags"})
		return
	}

	c.JSON(http.StatusOK, tags)
}
