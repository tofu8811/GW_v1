package proxy

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func prepareForwardedRequest(c *fiber.Ctx) {
	removeHopByHopHeaders(c)

	clientChain := c.IPs()
	if len(clientChain) == 0 {
		clientChain = []string{c.IP()}
	}

	c.Request().Header.Set("X-Forwarded-For", strings.Join(clientChain, ", "))
	c.Request().Header.Set("X-Forwarded-Proto", c.Protocol())
	c.Request().Header.Set("X-Forwarded-Host", string(c.Request().Host()))
	c.Request().Header.Set("X-Real-IP", c.IP())
}

func removeHopByHopHeaders(c *fiber.Ctx) {
	for _, header := range hopByHopHeaders {
		c.Request().Header.Del(header)
	}
}
