package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type leaderboardRepository struct {
	db  *sql.DB
	sql sqlExecutor
}

func NewLeaderboardRepository(sqlDB *sql.DB) service.LeaderboardRepository {
	return &leaderboardRepository{db: sqlDB, sql: sqlDB}
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
	conditions := "lud.usage_date < $1::date"
	args := []any{endTime}
	if window != service.LeaderboardWindowAllTime {
		conditions += " AND lud.usage_date >= $2::date"
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
				COALESCE(SUM(lud.requests), 0)::bigint AS requests,
				COALESCE(SUM(lud.tokens), 0)::bigint AS tokens,
				ROW_NUMBER() OVER (
					ORDER BY
						COALESCE(SUM(lud.tokens), 0) DESC,
						COALESCE(SUM(lud.requests), 0) DESC,
						lp.display_code ASC
				)::int AS rank
			FROM leaderboard_participants lp
			LEFT JOIN user_avatars ua ON ua.user_id = lp.user_id
			LEFT JOIN leaderboard_usage_daily lud ON lud.user_id = lp.user_id AND %s
			WHERE lp.is_opted_in = true AND lp.is_banned = false AND lp.opted_in_at IS NOT NULL
			GROUP BY lp.user_id, lp.display_name, lp.display_code
			HAVING COALESCE(SUM(lud.requests), 0) > 0
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
	defer func() { _ = rows.Close() }()

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

func (r *leaderboardRepository) GetHonorStats(ctx context.Context, userIDs []int64) (map[int64]map[string]service.LeaderboardHonorStats, error) {
	result := make(map[int64]map[string]service.LeaderboardHonorStats, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	for _, userID := range userIDs {
		result[userID] = emptyLeaderboardHonorStatsByWindow()
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			user_id,
			period_window,
			top_appearances,
			champion_count,
			runner_up_count,
			third_place_count,
			best_rank,
			current_streak,
			current_runner_up_streak,
			longest_runner_up_streak,
			perennial_runner_up
		FROM leaderboard_user_honors
		WHERE user_id = ANY($1)
	`, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var userID int64
		var periodWindow string
		var stats service.LeaderboardHonorStats
		stats.ChampionStarts = map[string][]time.Time{}
		if err := rows.Scan(
			&userID,
			&periodWindow,
			&stats.TopAppearances,
			&stats.ChampionCount,
			&stats.RunnerUpCount,
			&stats.ThirdPlaceCount,
			&stats.BestRank,
			&stats.CurrentStreak,
			&stats.CurrentRunnerUpStreak,
			&stats.LongestRunnerUpStreak,
			&stats.PerennialRunnerUp,
		); err != nil {
			return nil, err
		}
		if result[userID] == nil {
			result[userID] = emptyLeaderboardHonorStatsByWindow()
		}
		result[userID][periodWindow] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *leaderboardRepository) RebuildHonorStats(ctx context.Context, cutoff time.Time, latestCompleted map[string]time.Time) (int64, error) {
	if cutoff.IsZero() {
		cutoff = time.Now()
	}
	if r.db == nil {
		return r.rebuildHonorStatsWithExecutor(ctx, r.sql, cutoff, latestCompleted)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	affected, err := r.rebuildHonorStatsWithExecutor(ctx, tx, cutoff, latestCompleted)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return affected, nil
}

func (r *leaderboardRepository) rebuildHonorStatsWithExecutor(ctx context.Context, exec sqlExecutor, cutoff time.Time, latestCompleted map[string]time.Time) (int64, error) {
	if _, err := exec.ExecContext(ctx, `DELETE FROM leaderboard_user_honors`); err != nil {
		return 0, err
	}
	latestDaily := latestCompleted[service.LeaderboardWindowDaily]
	latestWeekly := latestCompleted[service.LeaderboardWindowWeekly]
	latestMonthly := latestCompleted[service.LeaderboardWindowMonthly]

	res, err := exec.ExecContext(ctx, `
		WITH visible_ranked AS (
			SELECT
				lpr.user_id,
				lpr.period_window,
				lpr.period_start,
				lpr.period_end,
				ROW_NUMBER() OVER (
					PARTITION BY lpr.period_window, lpr.period_start
					ORDER BY lpr.tokens DESC, lpr.requests DESC, lpr.user_id ASC
				)::int AS visible_rank
			FROM leaderboard_period_results lpr
			JOIN leaderboard_participants lp ON lp.user_id = lpr.user_id
			WHERE lp.is_opted_in = true
				AND lp.is_banned = false
				AND lp.opted_in_at IS NOT NULL
				AND lpr.period_window IN ('daily', 'weekly', 'monthly')
				AND lpr.period_end <= $1
		),
		honor_counts AS (
			SELECT
				user_id,
				period_window,
				COUNT(*) FILTER (WHERE visible_rank <= 10)::int AS top_appearances,
				COUNT(*) FILTER (WHERE visible_rank = 1)::int AS champion_count,
				COUNT(*) FILTER (WHERE visible_rank = 2)::int AS runner_up_count,
				COUNT(*) FILTER (WHERE visible_rank = 3)::int AS third_place_count,
				MIN(visible_rank) FILTER (WHERE visible_rank <= 10)::int AS best_rank
			FROM visible_ranked
			WHERE visible_rank <= 10
			GROUP BY user_id, period_window
		),
		champion_periods AS (
			SELECT user_id, period_window, period_start
			FROM visible_ranked
			WHERE visible_rank = 1
		),
		runner_up_periods AS (
			SELECT user_id, period_window, period_start
			FROM visible_ranked
			WHERE visible_rank = 2
		),
		latest_completed AS (
			SELECT
				'daily'::varchar AS period_window,
				$2::timestamptz AS latest_start,
				interval '1 day' AS step
			UNION ALL
			SELECT
				'weekly'::varchar,
				$3::timestamptz AS latest_start,
				interval '7 day' AS step
			UNION ALL
			SELECT
				'monthly'::varchar,
				$4::timestamptz AS latest_start,
				interval '1 month' AS step
		),
		champion_streaks AS (
			SELECT user_id, period_window, COUNT(*)::int AS current_streak
			FROM (
				SELECT
					cp.user_id,
					cp.period_window,
					cp.period_start,
					ROW_NUMBER() OVER (
						PARTITION BY cp.user_id, cp.period_window
						ORDER BY cp.period_start DESC
					)::int AS rn,
					lc.latest_start,
					lc.step
				FROM champion_periods cp
				JOIN latest_completed lc ON lc.period_window = cp.period_window
				WHERE cp.period_start <= lc.latest_start
			) ordered
			WHERE period_start = latest_start - ((rn - 1) * step)
			GROUP BY user_id, period_window
		),
		current_runner_up_streaks AS (
			SELECT user_id, period_window, COUNT(*)::int AS current_runner_up_streak
			FROM (
				SELECT
					rup.user_id,
					rup.period_window,
					rup.period_start,
					ROW_NUMBER() OVER (
						PARTITION BY rup.user_id, rup.period_window
						ORDER BY rup.period_start DESC
					)::int AS rn,
					lc.latest_start,
					lc.step
				FROM runner_up_periods rup
				JOIN latest_completed lc ON lc.period_window = rup.period_window
				WHERE rup.period_start <= lc.latest_start
			) ordered
			WHERE period_start = latest_start - ((rn - 1) * step)
			GROUP BY user_id, period_window
		),
		runner_up_islands AS (
			SELECT
				user_id,
				period_window,
				COUNT(*)::int AS streak_length
			FROM (
				SELECT
					rup.user_id,
					rup.period_window,
					rup.period_start,
					rup.period_start - ((ROW_NUMBER() OVER (
						PARTITION BY rup.user_id, rup.period_window
						ORDER BY rup.period_start
					)::int - 1) * lc.step) AS streak_key
				FROM runner_up_periods rup
				JOIN latest_completed lc ON lc.period_window = rup.period_window
			) islands
			GROUP BY user_id, period_window, streak_key
		),
		runner_up_streaks AS (
			SELECT
				user_id,
				period_window,
				MAX(streak_length)::int AS longest_runner_up_streak
			FROM runner_up_islands
			GROUP BY user_id, period_window
		),
		perennial_runner_up_leaders AS (
			SELECT
				user_id,
				period_window
			FROM (
				SELECT
					h.user_id,
					h.period_window,
					ROW_NUMBER() OVER (
						PARTITION BY h.period_window
						ORDER BY
							h.runner_up_count DESC,
							COALESCE(rus.longest_runner_up_streak, 0) DESC,
							h.best_rank ASC,
							h.top_appearances DESC,
							h.champion_count DESC,
							h.user_id ASC
					) AS rn
				FROM honor_counts h
				LEFT JOIN runner_up_streaks rus ON rus.user_id = h.user_id AND rus.period_window = h.period_window
				WHERE h.runner_up_count > 0
			) ranked_runner_ups
			WHERE rn = 1
		)
		INSERT INTO leaderboard_user_honors (
			user_id,
			period_window,
			top_appearances,
			champion_count,
			runner_up_count,
			third_place_count,
			best_rank,
			current_streak,
			current_runner_up_streak,
			longest_runner_up_streak,
			perennial_runner_up,
			updated_at
		)
		SELECT
			h.user_id,
			h.period_window,
			h.top_appearances,
			h.champion_count,
			h.runner_up_count,
			h.third_place_count,
			COALESCE(h.best_rank, 0),
			COALESCE(s.current_streak, 0),
			COALESCE(crus.current_runner_up_streak, 0),
			COALESCE(rus.longest_runner_up_streak, 0),
			COALESCE(pru.user_id IS NOT NULL, false),
			NOW()
		FROM honor_counts h
		LEFT JOIN champion_streaks s ON s.user_id = h.user_id AND s.period_window = h.period_window
		LEFT JOIN current_runner_up_streaks crus ON crus.user_id = h.user_id AND crus.period_window = h.period_window
		LEFT JOIN runner_up_streaks rus ON rus.user_id = h.user_id AND rus.period_window = h.period_window
		LEFT JOIN perennial_runner_up_leaders pru ON pru.user_id = h.user_id AND pru.period_window = h.period_window
	`, cutoff, latestDaily, latestWeekly, latestMonthly)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *leaderboardRepository) RebuildUsageDaily(ctx context.Context, startDate, endDate time.Time) (int64, error) {
	if !startDate.Before(endDate) {
		return 0, nil
	}
	if r.db == nil {
		return r.rebuildUsageDailyWithExecutor(ctx, r.sql, startDate, endDate)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	affected, err := r.rebuildUsageDailyWithExecutor(ctx, tx, startDate, endDate)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return affected, nil
}

func (r *leaderboardRepository) rebuildUsageDailyWithExecutor(ctx context.Context, exec sqlExecutor, startDate, endDate time.Time) (int64, error) {
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM leaderboard_usage_daily
		WHERE usage_date >= $1::date AND usage_date < $2::date
	`, startDate, endDate); err != nil {
		return 0, err
	}

	res, err := exec.ExecContext(ctx, `
		WITH daily AS (
			SELECT
				(ul.created_at AT TIME ZONE $3)::date AS usage_date,
				ul.user_id,
				COUNT(ul.id)::bigint AS requests,
				COALESCE(SUM(
					COALESCE(ul.input_tokens, 0) +
					COALESCE(ul.output_tokens, 0) +
					COALESCE(ul.cache_creation_tokens, 0) +
					COALESCE(ul.cache_read_tokens, 0) +
					COALESCE(ul.image_output_tokens, 0)
				), 0)::bigint AS tokens
			FROM usage_logs ul
			WHERE ul.created_at >= $1
				AND ul.created_at < $2
			GROUP BY (ul.created_at AT TIME ZONE $3)::date, ul.user_id
			HAVING COUNT(ul.id) > 0
		)
		INSERT INTO leaderboard_usage_daily (
			usage_date,
			user_id,
			tokens,
			requests,
			updated_at
		)
		SELECT usage_date, user_id, tokens, requests, NOW()
		FROM daily
		ON CONFLICT (usage_date, user_id) DO UPDATE SET
			tokens = EXCLUDED.tokens,
			requests = EXCLUDED.requests,
			updated_at = EXCLUDED.updated_at
	`, startDate, endDate, timezone.Name())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *leaderboardRepository) SnapshotPeriod(ctx context.Context, window string, startTime, endTime time.Time, limit int) (int64, error) {
	if r.db == nil {
		return r.snapshotPeriodWithExecutor(ctx, r.sql, window, startTime, endTime)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	inserted, err := r.snapshotPeriodWithExecutor(ctx, tx, window, startTime, endTime)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return inserted, nil
}

func (r *leaderboardRepository) snapshotPeriodWithExecutor(ctx context.Context, exec sqlExecutor, window string, startTime, endTime time.Time) (int64, error) {
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM leaderboard_period_results
		WHERE period_window = $1 AND period_start = $2
	`, window, startTime); err != nil {
		return 0, err
	}

	res, err := exec.ExecContext(ctx, `
		WITH ranked AS (
			SELECT
				ul.user_id,
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
						ul.user_id ASC
				)::int AS rank
			FROM usage_logs ul
			WHERE ul.created_at >= $2
				AND ul.created_at < $3
			GROUP BY ul.user_id
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
			NULL,
			UPPER(SUBSTRING(MD5(user_id::text), 1, 8)),
			tokens,
			requests
		FROM ranked
		ON CONFLICT DO NOTHING
	`, window, startTime, endTime)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func emptyLeaderboardHonorStatsByWindow() map[string]service.LeaderboardHonorStats {
	return map[string]service.LeaderboardHonorStats{
		service.LeaderboardWindowDaily:   {ChampionStarts: map[string][]time.Time{}},
		service.LeaderboardWindowWeekly:  {ChampionStarts: map[string][]time.Time{}},
		service.LeaderboardWindowMonthly: {ChampionStarts: map[string][]time.Time{}},
		service.LeaderboardWindowAllTime: {ChampionStarts: map[string][]time.Time{}},
	}
}

func (r *leaderboardRepository) scanParticipant(ctx context.Context, query string, args ...any) (*service.LeaderboardParticipant, error) {
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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
