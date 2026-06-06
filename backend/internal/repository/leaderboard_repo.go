package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type leaderboardRepository struct {
	sql sqlExecutor
}

func NewLeaderboardRepository(sqlDB *sql.DB) service.LeaderboardRepository {
	return &leaderboardRepository{sql: sqlDB}
}

func (r *leaderboardRepository) GetParticipant(ctx context.Context, userID int64) (*service.LeaderboardParticipant, error) {
	row, err := r.scanParticipant(ctx, `
		SELECT user_id, is_opted_in, is_banned, display_name, display_code, opted_in_at, created_at, updated_at
		FROM leaderboard_participants
		WHERE user_id = $1
	`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (r *leaderboardRepository) UpsertParticipant(ctx context.Context, input service.LeaderboardParticipantUpsert) (*service.LeaderboardParticipant, error) {
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	displayName := nullableString(input.DisplayName)
	optedInAt := sql.NullTime{Valid: input.IsOptedIn, Time: input.Now}

	row, err := r.scanParticipant(ctx, `
		INSERT INTO leaderboard_participants (
			user_id,
			is_opted_in,
			is_banned,
			display_name,
			display_code,
			opted_in_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, false, $3, $4, $5, $6, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			is_opted_in = CASE
				WHEN leaderboard_participants.is_banned = true THEN false
				ELSE EXCLUDED.is_opted_in
			END,
			display_name = CASE
				WHEN leaderboard_participants.is_banned = true THEN leaderboard_participants.display_name
				ELSE EXCLUDED.display_name
			END,
			display_code = leaderboard_participants.display_code,
			opted_in_at = CASE
				WHEN leaderboard_participants.is_banned = true THEN NULL
				WHEN EXCLUDED.is_opted_in = false THEN NULL
				WHEN leaderboard_participants.is_opted_in = true THEN leaderboard_participants.opted_in_at
				ELSE EXCLUDED.opted_in_at
			END,
			is_banned = leaderboard_participants.is_banned,
			updated_at = EXCLUDED.updated_at
		RETURNING user_id, is_opted_in, is_banned, display_name, display_code, opted_in_at, created_at, updated_at
	`, input.UserID, input.IsOptedIn, displayName, input.DisplayCode, optedInAt, input.Now)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (r *leaderboardRepository) RemoveParticipant(ctx context.Context, input service.LeaderboardParticipantRemove) (*service.LeaderboardParticipant, error) {
	if input.Now.IsZero() {
		input.Now = time.Now()
	}

	displayName := sql.NullString{}
	row, err := r.scanParticipant(ctx, `
		INSERT INTO leaderboard_participants (
			user_id,
			is_opted_in,
			is_banned,
			display_name,
			display_code,
			opted_in_at,
			created_at,
			updated_at
		)
		VALUES ($1, false, false, $2, $3, NULL, $4, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			is_opted_in = false,
			display_name = COALESCE(leaderboard_participants.display_name, EXCLUDED.display_name),
			display_code = COALESCE(leaderboard_participants.display_code, EXCLUDED.display_code),
			opted_in_at = NULL,
			updated_at = EXCLUDED.updated_at
		RETURNING user_id, is_opted_in, is_banned, display_name, display_code, opted_in_at, created_at, updated_at
	`, input.UserID, displayName, input.DisplayCode, input.Now)
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return row, nil
}

func (r *leaderboardRepository) SetParticipantBanStatus(ctx context.Context, input service.LeaderboardParticipantBanUpdate) (*service.LeaderboardParticipant, error) {
	if input.Now.IsZero() {
		input.Now = time.Now()
	}

	displayName := sql.NullString{}
	row, err := r.scanParticipant(ctx, `
		INSERT INTO leaderboard_participants (
			user_id,
			is_opted_in,
			is_banned,
			display_name,
			display_code,
			opted_in_at,
			created_at,
			updated_at
		)
		VALUES ($1, false, $2, $3, $4, NULL, $5, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			is_opted_in = false,
			is_banned = EXCLUDED.is_banned,
			display_name = COALESCE(leaderboard_participants.display_name, EXCLUDED.display_name),
			display_code = COALESCE(leaderboard_participants.display_code, EXCLUDED.display_code),
			opted_in_at = NULL,
			updated_at = EXCLUDED.updated_at
		RETURNING user_id, is_opted_in, is_banned, display_name, display_code, opted_in_at, created_at, updated_at
	`, input.UserID, input.IsBanned, displayName, input.DisplayCode, input.Now)
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return row, nil
}

func (r *leaderboardRepository) GetRanking(ctx context.Context, window string, startTime, endTime time.Time, limit int, currentUserID int64) ([]service.LeaderboardRankRow, *service.LeaderboardRankRow, error) {
	if limit <= 0 {
		limit = 10
	}
	conditions := "ul.created_at >= lp.opted_in_at AND ul.created_at < $1"
	args := []any{endTime}
	if window != service.LeaderboardWindowAllTime {
		conditions += " AND ul.created_at >= $2"
		args = append(args, startTime)
	}
	args = append(args, limit, currentUserID)
	limitPos := len(args) - 1
	userPos := len(args)

	query := fmt.Sprintf(`
		WITH ranked AS (
			SELECT
				lp.user_id,
				COALESCE(lp.display_name, '') AS display_name,
				lp.display_code,
				COALESCE(MAX(NULLIF(ua.url, '')), '') AS avatar_url,
				COUNT(ul.id)::bigint AS requests,
				COALESCE(SUM(
					COALESCE(ul.input_tokens, 0) +
					COALESCE(ul.output_tokens, 0) +
					COALESCE(ul.cache_creation_tokens, 0) +
					COALESCE(ul.cache_read_tokens, 0) +
					COALESCE(ul.image_output_tokens, 0)
				), 0)::bigint AS tokens,
				ROW_NUMBER() OVER (
					ORDER BY
						COALESCE(SUM(
							COALESCE(ul.input_tokens, 0) +
							COALESCE(ul.output_tokens, 0) +
							COALESCE(ul.cache_creation_tokens, 0) +
							COALESCE(ul.cache_read_tokens, 0) +
							COALESCE(ul.image_output_tokens, 0)
						), 0) DESC,
						COUNT(ul.id) DESC,
						lp.display_code ASC
				)::int AS rank
			FROM leaderboard_participants lp
			LEFT JOIN user_avatars ua ON ua.user_id = lp.user_id
			LEFT JOIN usage_logs ul ON ul.user_id = lp.user_id AND %s
			WHERE lp.is_opted_in = true AND lp.is_banned = false AND lp.opted_in_at IS NOT NULL
			GROUP BY lp.user_id, lp.display_name, lp.display_code
			HAVING COUNT(ul.id) > 0
		)
		SELECT user_id, rank, display_name, display_code, avatar_url, tokens, requests
		FROM ranked
		WHERE rank <= $%d OR user_id = $%d
		ORDER BY rank ASC, tokens DESC, requests DESC, display_code ASC
	`, conditions, limitPos, userPos)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	top := make([]service.LeaderboardRankRow, 0, limit)
	var me *service.LeaderboardRankRow
	for rows.Next() {
		var row service.LeaderboardRankRow
		if err := rows.Scan(&row.UserID, &row.Rank, &row.DisplayName, &row.DisplayCode, &row.AvatarURL, &row.Tokens, &row.Requests); err != nil {
			return nil, nil, err
		}
		if row.Rank <= limit {
			top = append(top, row)
		}
		if row.UserID == currentUserID {
			copied := row
			me = &copied
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return top, me, nil
}

func (r *leaderboardRepository) GetHonorStats(ctx context.Context, userIDs []int64) (map[int64]service.LeaderboardHonorStats, error) {
	result := make(map[int64]service.LeaderboardHonorStats, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	for _, userID := range userIDs {
		result[userID] = service.LeaderboardHonorStats{
			BestRank:       0,
			ChampionStarts: map[string][]time.Time{},
		}
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			user_id,
			COUNT(*)::int AS top_appearances,
			COUNT(*) FILTER (WHERE rank = 1)::int AS champion_count,
			MIN(rank)::int AS best_rank
		FROM leaderboard_period_results
		WHERE user_id = ANY($1)
		GROUP BY user_id
	`, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var stats service.LeaderboardHonorStats
		stats.ChampionStarts = map[string][]time.Time{}
		if err := rows.Scan(&userID, &stats.TopAppearances, &stats.ChampionCount, &stats.BestRank); err != nil {
			return nil, err
		}
		result[userID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.sql.QueryContext(ctx, `
		SELECT user_id, period_window, period_start
		FROM leaderboard_period_results
		WHERE user_id = ANY($1) AND rank = 1
		ORDER BY user_id ASC, period_window ASC, period_start DESC
	`, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var periodWindow string
		var start time.Time
		if err := rows.Scan(&userID, &periodWindow, &start); err != nil {
			return nil, err
		}
		stats := result[userID]
		if stats.ChampionStarts == nil {
			stats.ChampionStarts = map[string][]time.Time{}
		}
		stats.ChampionStarts[periodWindow] = append(stats.ChampionStarts[periodWindow], start)
		result[userID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *leaderboardRepository) SnapshotPeriod(ctx context.Context, window string, startTime, endTime time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 10
	}
	res, err := r.sql.ExecContext(ctx, `
		WITH ranked AS (
			SELECT
				lp.user_id,
				COALESCE(lp.display_name, '') AS display_name,
				lp.display_code,
				COUNT(ul.id)::bigint AS requests,
				COALESCE(SUM(
					COALESCE(ul.input_tokens, 0) +
					COALESCE(ul.output_tokens, 0) +
					COALESCE(ul.cache_creation_tokens, 0) +
					COALESCE(ul.cache_read_tokens, 0) +
					COALESCE(ul.image_output_tokens, 0)
				), 0)::bigint AS tokens,
				ROW_NUMBER() OVER (
					ORDER BY
						COALESCE(SUM(
							COALESCE(ul.input_tokens, 0) +
							COALESCE(ul.output_tokens, 0) +
							COALESCE(ul.cache_creation_tokens, 0) +
							COALESCE(ul.cache_read_tokens, 0) +
							COALESCE(ul.image_output_tokens, 0)
						), 0) DESC,
						COUNT(ul.id) DESC,
						lp.display_code ASC
				)::int AS rank
			FROM leaderboard_participants lp
			JOIN usage_logs ul ON ul.user_id = lp.user_id
				AND ul.created_at >= $2
				AND ul.created_at < $3
				AND ul.created_at >= lp.opted_in_at
			WHERE lp.is_opted_in = true AND lp.is_banned = false AND lp.opted_in_at IS NOT NULL
			GROUP BY lp.user_id, lp.display_name, lp.display_code
			HAVING COUNT(ul.id) > 0
		)
		INSERT INTO leaderboard_period_results (
			period_window,
			period_start,
			period_end,
			rank,
			user_id,
			display_name_snapshot,
			display_code_snapshot,
			tokens,
			requests
		)
		SELECT
			$1,
			$2,
			$3,
			rank,
			user_id,
			NULLIF(display_name, ''),
			display_code,
			tokens,
			requests
		FROM ranked
		WHERE rank <= $4
		ON CONFLICT DO NOTHING
	`, window, startTime, endTime, limit)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *leaderboardRepository) scanParticipant(ctx context.Context, query string, args ...any) (*service.LeaderboardParticipant, error) {
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	p, err := scanLeaderboardParticipant(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func scanLeaderboardParticipant(rows *sql.Rows) (*service.LeaderboardParticipant, error) {
	var p service.LeaderboardParticipant
	var displayName sql.NullString
	var optedInAt sql.NullTime
	if err := rows.Scan(&p.UserID, &p.IsOptedIn, &p.IsBanned, &displayName, &p.DisplayCode, &optedInAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if displayName.Valid {
		p.DisplayName = displayName.String
	}
	if optedInAt.Valid {
		p.OptedInAt = &optedInAt.Time
	}
	return &p, nil
}

func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23503"
	}
	return false
}
