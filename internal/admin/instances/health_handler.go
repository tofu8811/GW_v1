package instances

import (
	"errors"

	"gateway-api/helper/response"
	"gateway-api/helper/validation"
	upstreamhealth "gateway-api/internal/upstream/health"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) GetHealth(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if _, err := h.repository.FindByID(c.Context(), id); errors.Is(err, ErrInstanceNotFound) {
		return response.NotFound(c, "service instance not found")
	} else if err != nil {
		return response.InternalServerError(c)
	}
	if h.healthStore == nil {
		return response.InternalServerError(c)
	}

	ih, err := h.healthStore.GetInstanceHealth(c.Context(), id.String())
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, healthResponse(ih))
}

func (h *Handler) CheckHealth(c *fiber.Ctx) error {
	id, err := validation.ParseRequiredUUID("id", c.Params("id"))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if _, err := h.repository.FindByID(c.Context(), id); errors.Is(err, ErrInstanceNotFound) {
		return response.NotFound(c, "service instance not found")
	} else if err != nil {
		return response.InternalServerError(c)
	}
	if h.healthChecker == nil {
		return response.InternalServerError(c)
	}

	ih, err := h.healthChecker.CheckInstance(c.Context(), id.String())
	if errors.Is(err, upstreamhealth.ErrInstanceNotFound) {
		return response.NotFound(c, "service instance not found")
	}
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, healthResponse(ih))
}

func healthResponse(ih upstreamhealth.InstanceHealth) fiber.Map {
	return fiber.Map{
		"instance_id": ih.InstanceID,
		"service_id":  ih.ServiceID,
		"status":      ih.Status,
		"latency_ms":  ih.LatencyMS,
		"last_check":  ih.LastCheck,
		"fail_count":  ih.FailCount,
	}
}
