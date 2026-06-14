package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type groupRateTierRepository struct {
	sql sqlExecutor
	db  *sql.DB
}

func NewGroupRateTierRepository(sqlDB *sql.DB) *groupRateTierRepository {
	return &groupRateTierRepository{sql: sqlDB, db: sqlDB}
}

func (r *groupRateTierRepository) ListActiveByGroupID(ctx context.Context, groupID int64) ([]service.GroupRateTier, error) {
	return r.listByGroupID(ctx, groupID, true)
}

func (r *groupRateTierRepository) ListByGroupID(ctx context.Context, groupID int64) ([]service.GroupRateTier, error) {
	return r.listByGroupID(ctx, groupID, false)
}

func (r *groupRateTierRepository) listByGroupID(ctx context.Context, groupID int64, activeOnly bool) ([]service.GroupRateTier, error) {
	whereClause := "WHERE group_id = $1"
	if activeOnly {
		whereClause += " AND enabled = true"
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			id,
			group_id,
			tier_key,
			display_name,
			rate_multiplier,
			priority,
			enabled,
			is_default,
			fallback_policy,
			created_at,
			updated_at
		FROM group_rate_tiers
		`+whereClause+`
		ORDER BY is_default DESC, priority ASC, id ASC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tiers := make([]service.GroupRateTier, 0)
	for rows.Next() {
		var tier service.GroupRateTier
		var fallbackPolicyRaw []byte
		if err := rows.Scan(
			&tier.ID,
			&tier.GroupID,
			&tier.TierKey,
			&tier.DisplayName,
			&tier.RateMultiplier,
			&tier.Priority,
			&tier.Enabled,
			&tier.IsDefault,
			&fallbackPolicyRaw,
			&tier.CreatedAt,
			&tier.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tier.TierKey = strings.ToLower(strings.TrimSpace(tier.TierKey))
		tier.FallbackPolicy = map[string]any{}
		if len(fallbackPolicyRaw) > 0 {
			if err := json.Unmarshal(fallbackPolicyRaw, &tier.FallbackPolicy); err != nil {
				return nil, err
			}
		}
		tiers = append(tiers, tier)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tiers, nil
}

func (r *groupRateTierRepository) SyncGroupRateTiers(ctx context.Context, groupID int64, tiers []service.GroupRateTierInput) error {
	if r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM group_rate_tiers WHERE group_id = $1`, groupID); err != nil {
		return err
	}
	for _, tier := range tiers {
		policyRaw, marshalErr := json.Marshal(normalizeJSONMap(tier.FallbackPolicy))
		if marshalErr != nil {
			err = marshalErr
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO group_rate_tiers (
				group_id,
				tier_key,
				display_name,
				rate_multiplier,
				priority,
				enabled,
				is_default,
				fallback_policy,
				created_at,
				updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		`,
			groupID,
			strings.ToLower(strings.TrimSpace(tier.TierKey)),
			strings.TrimSpace(tier.DisplayName),
			tier.RateMultiplier,
			tier.Priority,
			tier.Enabled,
			tier.IsDefault,
			policyRaw,
		)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (r *groupRateTierRepository) RecordGroupTierHealthEvent(ctx context.Context, event service.GroupTierHealthEvent) error {
	if r == nil || r.sql == nil {
		return nil
	}
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var observedTTFT any
	if event.ObservedTTFTMs != nil {
		observedTTFT = *event.ObservedTTFTMs
	}
	_, err = r.sql.ExecContext(ctx, `
		INSERT INTO group_tier_health_events (
			group_id,
			tier_key,
			model_key,
			capability,
			old_state,
			new_state,
			reason,
			observed_ttft_ms,
			sample_count,
			metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`,
		event.GroupID,
		strings.ToLower(strings.TrimSpace(event.TierKey)),
		strings.ToLower(strings.TrimSpace(event.ModelKey)),
		strings.TrimSpace(event.Capability),
		strings.TrimSpace(event.OldState),
		strings.TrimSpace(event.NewState),
		strings.TrimSpace(event.Reason),
		observedTTFT,
		event.SampleCount,
		metadataRaw,
	)
	return err
}
