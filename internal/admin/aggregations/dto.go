package aggregations

import (
	"encoding/json"
	"time"
)

type CreateAggregationRequest struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Method   string `json:"method"`
	IsActive *bool  `json:"is_active"`
}

type UpdateAggregationRequest struct {
	Name     *string `json:"name"`
	Path     *string `json:"path"`
	Method   *string `json:"method"`
	IsActive *bool   `json:"is_active"`
}

type AggregationResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Method    string    `json:"method"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateAggregationStepRequest struct {
	ServiceID       string          `json:"service_id"`
	Sequence        int16           `json:"sequence"`
	DependsOn       *string         `json:"depends_on"`
	IsRequired      *bool           `json:"is_required"`
	RequestTemplate json.RawMessage `json:"request_template"`
	ResponseMapping json.RawMessage `json:"response_mapping"`
	IsActive        *bool           `json:"is_active"`
}

type UpdateAggregationStepRequest struct {
	ServiceID       *string         `json:"service_id"`
	Sequence        *int16          `json:"sequence"`
	DependsOn       NullableUUID    `json:"depends_on"`
	IsRequired      *bool           `json:"is_required"`
	RequestTemplate json.RawMessage `json:"request_template"`
	ResponseMapping json.RawMessage `json:"response_mapping"`
	IsActive        *bool           `json:"is_active"`
}

type NullableUUID struct {
	Set   bool
	Value *string
}

func (f *NullableUUID) UnmarshalJSON(data []byte) error {
	f.Set = true
	if string(data) == "null" {
		f.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	f.Value = &value
	return nil
}

type AggregationStepResponse struct {
	ID              string          `json:"id"`
	AggregationID   string          `json:"aggregation_id"`
	ServiceID       string          `json:"service_id"`
	Sequence        int16           `json:"sequence"`
	DependsOn       *string         `json:"depends_on"`
	IsRequired      bool            `json:"is_required"`
	RequestTemplate json.RawMessage `json:"request_template"`
	ResponseMapping json.RawMessage `json:"response_mapping"`
	IsActive        bool            `json:"is_active"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
