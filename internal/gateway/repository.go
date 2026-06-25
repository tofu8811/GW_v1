package gateway

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindCandidates(ctx context.Context, method string) ([]UpstreamRoute, error) {
	query := `
		SELECT
			r.path,
			r.method,
			r.strip_prefix,
			r.rewrite_target,
			s.name,
			s.protocol,
			si.host,
			si.port,
			s.timeout_ms
		FROM routes r
		JOIN services s ON s.id = r.service_id
		JOIN service_instances si ON si.service_id = s.id
		WHERE r.is_active = TRUE
		  AND s.is_active = TRUE
		  AND si.is_active = TRUE
		  AND s.protocol = 'http'
		  AND (r.method = $1 OR r.method = 'ANY')
		ORDER BY r.priority DESC, length(r.path) DESC, r.created_at DESC, si.created_at ASC
	`

	rows, err := r.db.Query(ctx, query, method)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []UpstreamRoute
	for rows.Next() {
		var route UpstreamRoute
		if err := rows.Scan(
			&route.RoutePath,
			&route.RouteMethod,
			&route.StripPrefix,
			&route.RewriteTarget,
			&route.ServiceName,
			&route.Protocol,
			&route.Host,
			&route.Port,
			&route.TimeoutMS,
		); err != nil {
			return nil, err
		}

		routes = append(routes, route)
	}

	return routes, rows.Err()
}
