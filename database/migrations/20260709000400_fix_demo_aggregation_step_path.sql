-- +goose Up
UPDATE aggregation_steps st
SET request_template = jsonb_set(st.request_template, '{path}', '"/api/products"'::jsonb),
    updated_at = now()
FROM aggregation_configs agg
WHERE st.aggregation_id = agg.id
  AND agg.path = '/api/dashboard'
  AND agg.method = 'GET'
  AND st.request_template ->> 'path' = '/products'
  AND st.deleted_at IS NULL;

-- +goose Down
UPDATE aggregation_steps st
SET request_template = jsonb_set(st.request_template, '{path}', '"/products"'::jsonb),
    updated_at = now()
FROM aggregation_configs agg
WHERE st.aggregation_id = agg.id
  AND agg.path = '/api/dashboard'
  AND agg.method = 'GET'
  AND st.request_template ->> 'path' = '/api/products'
  AND st.deleted_at IS NULL;