package server

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/gin-gonic/gin"
)

type queryReq struct {
	Query string `json:"query"`
}

// Query performs graphql related tasks
func Query(c *gin.Context) {
	var query queryReq
	if bindErr := c.ShouldBindJSON(&query); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	results, queryErr := db.Query(query.Query)
	if queryErr != nil {
		log.Error().Msg(queryErr.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": queryErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": results,
	})
}
