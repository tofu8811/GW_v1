package services

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
	if _, err := h.repository.FindByID(c.Context(), id); errors.Is(err, ErrServiceNotFound) {
		return response.NotFound(c, "service not found")
	} else if err != nil {
		return response.InternalServerError(c)
	}
	if h.healthStore == nil || h.configCache == nil {
		return response.InternalServerError(c)
	}

	instances := h.configCache.ActiveInstancesByService(id.String())
	aliveSet, err := h.healthStore.AliveSet(c.Context(), id.String())
	if err != nil {
		return response.InternalServerError(c)
	}

	items := make([]fiber.Map, 0, len(instances))
	aliveCount := 0
	downCount := 0
	for _, instance := range instances {
		ih, err := h.healthStore.GetInstanceHealth(c.Context(), instance.InstanceID)
		if err != nil {
			return response.InternalServerError(c)
		}
		if _, ok := aliveSet[instance.InstanceID]; ok {
			aliveCount++
		} else if ih.Status == upstreamhealth.StatusDown {
			downCount++
		}
		items = append(items, fiber.Map{
			"instance_id": instance.InstanceID,
			"health_path": instance.HealthPath,
			"status":      ih.Status,
			"latency_ms":  ih.LatencyMS,
			"last_check":  ih.LastCheck,
			"fail_count":  ih.FailCount,
		})
	}

	return response.OK(c, fiber.Map{
		"service_id": id.String(),
		"total":      len(instances),
		"alive":      aliveCount,
		"down":       downCount,
		"instances":  items,
	})
}
