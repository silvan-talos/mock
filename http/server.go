package http

import (
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/silvan-talos/mock/mocking"
)

type Server struct {
	ms     mocking.Service
	router *gin.Engine
}

func NewServer(ms mocking.Service) Server {
	s := Server{
		ms: ms,
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/mock", s.mockInterface)
	s.router = r
	return s
}

func (s *Server) Serve(lis net.Listener) error {
	log.Println("Starting http server", "address", lis.Addr().String())
	return s.router.RunListener(lis)
}

func (s *Server) mockInterface(c *gin.Context) {
	err := s.ms.ProcessOne(c.Request.Body, c.Writer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}
