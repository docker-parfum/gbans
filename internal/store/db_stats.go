package store

import (
	"context"
	sq "github.com/Masterminds/squirrel"
	"github.com/leighmacdonald/gbans/internal/config"
	"github.com/leighmacdonald/gbans/internal/model"
	"github.com/leighmacdonald/gbans/pkg/fp"
	"github.com/leighmacdonald/gbans/pkg/logparse"
	"github.com/leighmacdonald/steamid/v2/steamid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

func (database *pgStore) MatchSave(ctx context.Context, match *model.Match) error {
	for _, p := range match.PlayerSums {
		var player model.Person
		if errPlayer := database.GetOrCreatePersonBySteamID(ctx, p.SteamId, &player); errPlayer != nil {
			return errors.Wrapf(errPlayer, "Failed to create person")
		}
	}
	const q = `INSERT INTO match (server_id, map, created_on, title) VALUES ($1, $2, $3, $4) RETURNING match_id`
	if errMatch := database.QueryRow(ctx, q, match.ServerId, match.MapName, match.CreatedOn, match.Title).
		Scan(&match.MatchID); errMatch != nil {
		return errors.Wrapf(errMatch, "Failed to setup match")
	}
	const pq = `INSERT INTO match_player (
			match_id, steam_id, team, 
			time_start, time_end, kills, assists, deaths, dominations, dominated, 
			revenges, damage, damage_taken, healing, healing_taken, health_packs, 
			backstabs, headshots, airshots, captures, shots, extinguishes, 
			hits, buildings, buildings_destroyed) 
		VALUES (
			$1,  $2, $3, 
		    $4,  $5, $6, $7, $8, $9, $10, 
		    $11, $12, $13, $14, $15, $16, 
		    $17, $18, $19, $20, $21, $22, 
		    $23, $24, $25
		) RETURNING match_player_id`
	for _, s := range match.PlayerSums {
		endTime := &match.CreatedOn
		if s.TimeEnd != nil {
			// Use match end time
			endTime = s.TimeEnd
		}
		if errPlayerExec := database.QueryRow(ctx, pq, match.MatchID, s.SteamId, s.Team, s.TimeStart, endTime, s.Kills, s.Assists, s.Deaths, s.Dominations, s.Dominated, s.Revenges, s.Damage, s.DamageTaken, s.Healing, s.HealingTaken, s.HealthPacks, s.BackStabs, s.HeadShots, s.Airshots, s.Captures, s.Shots, s.Extinguishes, s.Hits, s.BuildingDestroyed, s.BuildingDestroyed).Scan(&s.MatchPlayerSumID); errPlayerExec != nil {
			return errors.Wrapf(errPlayerExec, "Failed to write player sum")
		}
	}

	const mq = `INSERT INTO match_medic (
            match_id, steam_id, healing, charges, drops, avg_time_to_build, avg_time_before_use, 
            near_full_charge_death, avg_uber_length, death_after_charge, major_adv_lost, biggest_adv_lost) 
            VALUES ($1, $2, $3, $4, $5,$6, $7, $8, $9, $10,$11, $12) RETURNING match_medic_id`

	for _, s := range match.MedicSums {
		charges := 0
		for _, mg := range s.Charges {
			charges += mg
		}
		if errMedExec := database.QueryRow(ctx, mq, match.MatchID, s.SteamId, s.Healing, charges, s.Drops, s.AvgTimeToBuild, s.AvgTimeBeforeUse, s.NearFullChargeDeath, s.AvgUberLength, s.DeathAfterCharge, s.MajorAdvLost, s.BiggestAdvLost).Scan(&s.MatchMedicId); errMedExec != nil {
			return errors.Wrapf(errMedExec, "Failed to write medic sum")
		}
	}

	const tq = `INSERT INTO match_team (
		match_id, team, kills, damage, charges, drops, caps, mid_fights
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING match_team_id`
	// FIXME team value unset
	for i, s := range match.TeamSums {
		if errTeamExec := database.QueryRow(ctx, tq, match.MatchID, i+1, s.Kills, s.Damage, s.Charges, s.Drops, s.Caps, s.MidFights).Scan(&s.MatchTeamId); errTeamExec != nil {
			return errors.Wrapf(errTeamExec, "Failed to write team sum")
		}
	}
	return nil
}

type MatchesQueryOpts struct {
	QueryFilter
	SteamID   steamid.SID64 `json:"steam_id"`
	ServerId  int           `json:"server_id"`
	Map       string        `json:"map"`
	TimeStart *time.Time    `json:"time_start,omitempty"`
	TimeEnd   *time.Time    `json:"time_end,omitempty"`
}

func (database *pgStore) Matches(ctx context.Context, opts MatchesQueryOpts) (model.MatchSummaryCollection, error) {
	qb := sb.Select("m.match_id", "m.server_id", "m.map", "m.created_on", "COALESCE(sum(mp.kills), 0)", "COALESCE(sum(mp.assists), 0)", "COALESCE(sum(mp.damage), 0)", "COALESCE(sum(mp.healing), 0)", "COALESCE(sum(mp.airshots), 0)").
		From("match m").
		LeftJoin("match_player mp on m.match_id = mp.match_id").
		GroupBy("m.match_id")
	if opts.Map != "" {
		qb = qb.Where(sq.Eq{"m.map_name": opts.Map})
	}
	if opts.SteamID > 0 {
		qb = qb.Where(sq.Eq{"mp.steam_id": opts.SteamID})
	}
	if opts.SortDesc {
		qb = qb.OrderBy("m.match_id DESC")
	} else {
		qb = qb.OrderBy("m.match_id ASC")
	}
	if opts.Limit > 0 {
		qb = qb.Limit(opts.Limit)
	}
	query, args, errQueryArgs := qb.ToSql()
	if errQueryArgs != nil {
		return nil, errors.Wrapf(errQueryArgs, "Failed to build query")
	}
	rows, errQuery := database.Query(ctx, query, args...)
	if errQuery != nil {
		return nil, errors.Wrapf(errQuery, "Failed to query matches")
	}
	defer rows.Close()
	var matches model.MatchSummaryCollection
	for rows.Next() {
		var m model.MatchSummary
		if errScan := rows.Scan(&m.MatchID, &m.ServerId, &m.MapName, &m.CreatedOn /*&m.PlayerCount,*/, &m.Kills, &m.Assists, &m.Damage, &m.Healing, &m.Airshots); errScan != nil {
			return nil, errors.Wrapf(errScan, "Failed to scan match row")
		}
		matches = append(matches, &m)
	}
	return matches, nil
}

func (database *pgStore) MatchGetById(ctx context.Context, matchId int) (*model.Match, error) {
	m := model.NewMatch()
	m.MatchID = matchId
	const qm = `SELECT server_id, map, title, created_on  FROM match WHERE match_id = $1`
	if errMatch := database.QueryRow(ctx, qm, matchId).Scan(&m.ServerId, &m.MapName, &m.Title, &m.CreatedOn); errMatch != nil {
		return nil, errors.Wrapf(errMatch, "Failed to load root match")
	}
	const qp = `
		SELECT 
		    match_player_id, steam_id, team, time_start, time_end, kills, assists,
       		deaths, dominations, dominated, revenges, damage, damage_taken, healing, healing_taken, health_packs, 
       		backstabs, headshots, airshots, captures, shots, extinguishes, hits, buildings, 
       		buildings_destroyed, (kills::real/deaths::real), ((kills::real+assists::real)/deaths::real)
		FROM 
		    match_player
		WHERE 
		    match_id = $1`
	playerRows, errPlayer := database.Query(ctx, qp, matchId)
	if errPlayer != nil {
		return nil, errors.Wrapf(errPlayer, "Failed to query match players")
	}
	defer playerRows.Close()
	for playerRows.Next() {
		s := model.MatchPlayerSum{MatchPlayerSumID: matchId}
		if errRow := playerRows.Scan(&s.MatchPlayerSumID, &s.SteamId, &s.Team, &s.TimeStart, &s.TimeEnd, &s.Kills, &s.Assists, &s.Deaths, &s.Dominations, &s.Dominated, &s.Revenges, &s.Damage, &s.DamageTaken, &s.Healing, &s.HealingTaken, &s.HealthPacks, &s.BackStabs, &s.HeadShots, &s.Airshots, &s.Captures, &s.Shots, &s.Extinguishes, &s.Hits, &s.BuildingBuilt, &s.BuildingDestroyed, &s.KDRatio, &s.KADRatio); errRow != nil {
			return nil, errors.Wrapf(errPlayer, "Failed to scan match players")
		}
		m.PlayerSums = append(m.PlayerSums, &s)
	}
	const qMed = `
		SELECT 
		    match_medic_id, steam_id, healing, charges, drops, avg_time_to_build, 
       		avg_time_before_use, near_full_charge_death, avg_uber_length, death_after_charge, 
       		major_adv_lost, biggest_adv_lost 
		FROM
		    match_medic
		WHERE 
		    match_id = $1`
	medicRows, errMedQuery := database.Query(ctx, qMed, matchId)
	if errMedQuery != nil && !errors.Is(errMedQuery, ErrNoResult) {
		return nil, errors.Wrapf(errMedQuery, "Failed to query match medics")
	}
	defer medicRows.Close()
	for medicRows.Next() {
		ms := model.MatchMedicSum{MatchId: matchId, Charges: map[logparse.Medigun]int{
			logparse.Uber:       0,
			logparse.Kritzkrieg: 0,
			logparse.Vaccinator: 0,
			logparse.QuickFix:   0,
		}}
		charges := 0
		if errRow := medicRows.Scan(&ms.MatchMedicId, &ms.SteamId, &ms.Healing, &charges, &ms.Drops, &ms.AvgTimeToBuild, &ms.AvgTimeBeforeUse, &ms.NearFullChargeDeath, &ms.AvgUberLength, &ms.DeathAfterCharge, &ms.MajorAdvLost, &ms.BiggestAdvLost); errRow != nil {
			return nil, errors.Wrapf(errMedQuery, "Failed to scan match medics")
		}
		// FIXME all charges are counted as uber for now
		ms.Charges[logparse.Uber] = charges
		m.MedicSums = append(m.MedicSums, &ms)
	}

	const qTeam = `
		SELECT 
		    match_team_id, team, kills, damage, charges, drops, caps, mid_fights 
		FROM 
		    match_team 
		WHERE 
		    match_id = $1`
	teamRows, errTeamQuery := database.Query(ctx, qTeam, matchId)
	if errTeamQuery != nil && !errors.Is(errTeamQuery, ErrNoResult) {
		return nil, errors.Wrapf(errMedQuery, "Failed to query match medics")
	}
	defer teamRows.Close()
	for teamRows.Next() {
		ts := model.MatchTeamSum{MatchId: matchId}
		if errRow := teamRows.Scan(&ts.MatchTeamId, &ts.Team, &ts.Kills, &ts.Damage, &ts.Charges, &ts.Drops, &ts.Caps, &ts.MidFights); errRow != nil {
			return nil, errors.Wrapf(errRow, "Failed to scan match medics")
		}
		m.TeamSums = append(m.TeamSums, &ts)
	}
	var ids steamid.Collection
	for _, p := range m.PlayerSums {
		ids = append(ids, p.SteamId)
	}
	players, errPlayers := database.GetPeopleBySteamID(ctx, ids)
	if errPlayers != nil {
		return nil, errors.Wrapf(errPlayers, "Failed to load players")
	}
	m.Players = players
	return &m, nil
}

func (database *pgStore) GetStats(ctx context.Context, stats *model.Stats) error {
	const q = `
	SELECT 
		(SELECT COUNT(ban_id) FROM ban) as bans_total,
		(SELECT COUNT(ban_id) FROM ban WHERE created_on >= (now() - INTERVAL '1 DAY')) as bans_day,
	    (SELECT COUNT(ban_id) FROM ban WHERE created_on >= (now() - INTERVAL '1 DAY')) as bans_week,
		(SELECT COUNT(ban_id) FROM ban WHERE created_on >= (now() - INTERVAL '1 MONTH')) as bans_month, 
	    (SELECT COUNT(ban_id) FROM ban WHERE created_on >= (now() - INTERVAL '3 MONTH')) as bans_3month,
	    (SELECT COUNT(ban_id) FROM ban WHERE created_on >= (now() - INTERVAL '6 MONTH')) as bans_6month,
	    (SELECT COUNT(ban_id) FROM ban WHERE created_on >= (now() - INTERVAL '1 YEAR')) as bans_year,
		(SELECT COUNT(net_id) FROM ban_net) as bans_cidr,
		(SELECT COUNT(word_id) FROM filtered_word) as filtered_words,
		(SELECT COUNT(server_id) FROM server) as servers_total`
	if errQuery := database.conn.QueryRow(ctx, q).
		Scan(&stats.BansTotal, &stats.BansDay, &stats.BansWeek, &stats.BansMonth, &stats.Bans3Month, &stats.Bans6Month, &stats.BansYear, &stats.BansCIDRTotal, &stats.FilteredWords, &stats.ServersTotal); errQuery != nil {
		log.Errorf("Failed to fetch stats: %v", errQuery)
		return Err(errQuery)
	}
	return nil
}

var localStatColumns = []string{"players", "capacity_full", "capacity_empty", "capacity_partial",
	"map_types", "created_on", "regions", "servers"}

func (database *pgStore) SaveLocalTF2Stats(ctx context.Context, duration StatDuration, stats model.LocalTF2StatsSnapshot) error {
	query, args, errQuery := sb.Insert(statDurationTable(Local, duration)).
		Columns(localStatColumns...).
		Values(stats.Players, stats.CapacityFull, stats.CapacityEmpty, stats.CapacityPartial,
			stats.MapTypes, stats.CreatedOn, stats.Regions, stats.Servers).
		ToSql()
	if errQuery != nil {
		return errQuery
	}
	return Err(database.Exec(ctx, query, args...))
}

var globalStatColumns = []string{"players", "bots", "secure", "servers_community", "servers_total",
	"capacity_full", "capacity_empty", "capacity_partial", "map_types", "created_on", "regions"}

func (database *pgStore) SaveGlobalTF2Stats(ctx context.Context, duration StatDuration, stats model.GlobalTF2StatsSnapshot) error {
	query, args, errQuery := sb.Insert(statDurationTable(Global, duration)).
		Columns(globalStatColumns...).
		Values(stats.Players, stats.Bots, stats.Secure, stats.ServersCommunity, stats.ServersTotal, stats.CapacityFull, stats.CapacityEmpty, stats.CapacityPartial, stats.TrimMapTypes(), stats.CreatedOn, stats.Regions).
		ToSql()
	if errQuery != nil {
		return errQuery
	}
	return Err(database.Exec(ctx, query, args...))
}

type StatLocality int

const (
	Local StatLocality = iota
	Global
)

type StatDuration int

const (
	Live StatDuration = iota
	Hourly
	Daily
	Weekly
	Yearly
)

func fetchGlobalTF2Snapshots(ctx context.Context, database Store, query string, args []any) ([]model.GlobalTF2StatsSnapshot, error) {
	rows, errExec := database.Query(ctx, query, args...)
	if errExec != nil {
		return nil, Err(errExec)
	}
	defer rows.Close()
	var stats []model.GlobalTF2StatsSnapshot
	for rows.Next() {
		var stat model.GlobalTF2StatsSnapshot
		if errScan := rows.Scan(&stat.StatId, &stat.Players, &stat.Bots, &stat.Secure, &stat.ServersCommunity, &stat.ServersTotal, &stat.CapacityFull, &stat.CapacityEmpty, &stat.CapacityPartial, &stat.MapTypes, &stat.CreatedOn, &stat.Regions); errScan != nil {
			return nil, Err(errScan)
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func fetchLocalTF2Snapshots(ctx context.Context, database Store, query string, args []any) ([]model.LocalTF2StatsSnapshot, error) {
	rows, errExec := database.Query(ctx, query, args...)
	if errExec != nil {
		return nil, Err(errExec)
	}
	defer rows.Close()
	var stats []model.LocalTF2StatsSnapshot
	for rows.Next() {
		var stat model.LocalTF2StatsSnapshot
		if errScan := rows.Scan(&stat.StatId, &stat.Players, &stat.CapacityFull, &stat.CapacityEmpty,
			&stat.CapacityPartial, &stat.MapTypes, &stat.CreatedOn, &stat.Regions, &stat.Servers); errScan != nil {
			return nil, Err(errScan)
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func HourlyIndex(t time.Time) (time.Time, int) {
	curYear, curMon, curDay := t.Date()
	curHour, _, _ := t.Clock()
	curTime := time.Date(curYear, curMon, curDay, curHour, 0, 0, 0, t.Location())
	return curTime, curHour
}

func DailyIndex(t time.Time) (time.Time, int) {
	curYear, curMon, curDay := t.Date()
	curTime := time.Date(curYear, curMon, curDay, 0, 0, 0, 0, t.Location())
	return curTime, curDay
}

// currentHourlyTime calculates the absolute start of the current hour
func currentHourlyTime() time.Time {
	now := config.Now()
	year, mon, day := now.Date()
	hour, _, _ := now.Clock()
	return time.Date(year, mon, day, hour, 0, 0, 0, now.Location())
}

// currentDailyTime calculates the absolute start of the current day
func currentDailyTime() time.Time {
	now := config.Now()
	year, mon, day := now.Date()
	return time.Date(year, mon, day, 0, 0, 0, 0, now.Location())
}

type statIndexFunc = func(t time.Time) (time.Time, int)

func (database *pgStore) BuildGlobalTF2Stats(ctx context.Context) error {
	maxDate := currentHourlyTime()
	query, args, errQuery := sb.
		Select(fp.Prepend(globalStatColumns, "stat_id")...).
		From(statDurationTable(Global, Live)).
		Where(sq.Lt{"created_on": maxDate}). // Ignore any results until a full hour has passed
		OrderBy("created_on").
		ToSql()
	if errQuery != nil {
		return Err(errQuery)
	}
	stats, errStats := fetchGlobalTF2Snapshots(ctx, database, query, args)
	if errStats != nil {
		return errStats
	}
	if len(stats) == 0 {
		return nil
	}
	var (
		hourlySums           []model.GlobalTF2StatsSnapshot
		curSums              *model.GlobalTF2StatsSnapshot
		tempPlayers          []int
		tempBots             []int
		tempServersCommunity []int
		tempServersTotal     []int
		tempCapacityFull     []int
		tempCapacityEmpty    []int
		tempCapacityPartial  []int
	)
	tempMapTypes := map[string][]int{}
	tempRegions := map[string][]int{}
	curIdx := 0
	for _, s := range stats {
		// Group & sum hourly as the minimum granularity
		curTime, timeIdx := HourlyIndex(s.CreatedOn)
		if curIdx == 0 {
			curIdx = timeIdx
		} else if curIdx != timeIdx && curSums != nil {
			// If the hour index changed, flush the current results out
			sumStat := model.NewGlobalTF2Stats()
			sumStat.CreatedOn = curTime
			sumStat.Players = fp.Avg(tempPlayers)
			sumStat.Bots = fp.Avg(tempBots)
			sumStat.ServersCommunity = fp.Avg(tempServersCommunity)
			sumStat.ServersTotal = fp.Avg(tempServersTotal)
			sumStat.CapacityFull = fp.Avg(tempCapacityFull)
			sumStat.CapacityEmpty = fp.Avg(tempCapacityEmpty)
			sumStat.CapacityPartial = fp.Avg(tempCapacityPartial)
			for k := range tempMapTypes {
				sumStat.MapTypes[k] = fp.Avg(tempMapTypes[k])
			}
			for k := range tempRegions {
				sumStat.Regions[k] = fp.Avg(tempRegions[k])
			}
			hourlySums = append(hourlySums, sumStat)
			curSums = nil
			curIdx = timeIdx
			tempPlayers = nil
			tempBots = nil
			tempServersCommunity = nil
			tempServersTotal = nil
			tempCapacityFull = nil
			tempCapacityEmpty = nil
			tempCapacityPartial = nil
			tempRegions = map[string][]int{}
			tempMapTypes = map[string][]int{}
		}
		if curSums == nil {
			curSums = &model.GlobalTF2StatsSnapshot{CreatedOn: curTime}
		}
		tempPlayers = append(tempPlayers, s.Players)
		tempBots = append(tempBots, s.Bots)
		tempServersCommunity = append(tempServersCommunity, s.ServersCommunity)
		tempServersTotal = append(tempServersTotal, s.ServersTotal)
		tempCapacityFull = append(tempCapacityFull, s.CapacityFull)
		tempCapacityEmpty = append(tempCapacityEmpty, s.CapacityEmpty)
		tempCapacityPartial = append(tempCapacityPartial, s.CapacityPartial)
		for k, v := range s.MapTypes {
			_, found := tempMapTypes[k]
			if !found {
				tempMapTypes[k] = []int{}
			}
			tempMapTypes[k] = append(tempMapTypes[k], v)
		}
		for k, v := range s.Regions {
			_, found := tempRegions[k]
			if !found {
				tempRegions[k] = []int{}
			}
			tempRegions[k] = append(tempRegions[k], v)
		}
	}
	for _, hourly := range hourlySums {
		if errSave := database.SaveGlobalTF2Stats(ctx, Hourly, hourly); errSave != nil {
			if errors.Is(errSave, ErrDuplicate) {
				continue
			}
			return errSave
		}
	}

	var statIds []int64
	for _, s := range stats {
		statIds = append(statIds, s.StatId)
	}
	// Delete old entries
	delQuery, delArgs, delQueryErr := sb.
		Delete(statDurationTable(Global, Live)).
		Where(sq.Eq{"stat_id": statIds}).
		ToSql()
	if delQueryErr != nil {
		return Err(delQueryErr)
	}
	return database.Exec(ctx, delQuery, delArgs...)
}

func statDurationTable(locality StatLocality, duration StatDuration) string {
	switch locality {
	case Global:
		switch duration {
		case Hourly:
			return "global_stats_players_hourly"
		case Daily:
			return "global_stats_players_daily"
		default:
			return "global_stats_players"
		}
	default:
		switch duration {
		case Hourly:
			return "local_stats_players_hourly"
		case Daily:
			return "local_stats_players_daily"
		default:
			return "local_stats_players"
		}
	}
}

func (database *pgStore) GetGlobalTF2Stats(ctx context.Context, duration StatDuration) ([]model.GlobalTF2StatsSnapshot, error) {
	table := statDurationTable(Global, duration)
	if table == "" {
		return nil, errors.New("Unsupported stat duration")
	}
	qb := sb.Select(fp.Prepend(globalStatColumns, "stat_id")...).
		From(table).
		OrderBy("created_on desc")
	switch duration {
	case Hourly:
		qb = qb.Limit(24 * 7)
	}
	query, args, errQuery := qb.ToSql()
	if errQuery != nil {
		return nil, Err(errQuery)
	}
	return fetchGlobalTF2Snapshots(ctx, database, query, args)
}

func (database *pgStore) GetLocalTF2Stats(ctx context.Context, duration StatDuration) ([]model.LocalTF2StatsSnapshot, error) {
	table := statDurationTable(Local, duration)
	if table == "" {
		return nil, errors.New("Unsupported stat duration")
	}
	qb := sb.Select(fp.Prepend(localStatColumns, "stat_id")...).
		From(table).
		OrderBy("created_on desc")
	switch duration {
	case Hourly:
		qb = qb.Limit(24 * 7)
	}
	query, args, errQuery := qb.ToSql()
	if errQuery != nil {
		return nil, Err(errQuery)
	}
	return fetchLocalTF2Snapshots(ctx, database, query, args)
}

func (database *pgStore) BuildLocalTF2Stats(ctx context.Context) error {
	maxDate := currentHourlyTime()
	query, args, errQuery := sb.
		Select(fp.Prepend(localStatColumns, "stat_id")...).
		From(statDurationTable(Local, Live)).
		Where(sq.Lt{"created_on": maxDate}). // Ignore any results until a full hour has passed
		OrderBy("created_on").
		ToSql()
	if errQuery != nil {
		return Err(errQuery)
	}
	stats, errStats := fetchLocalTF2Snapshots(ctx, database, query, args)
	if errStats != nil {
		return errStats
	}
	if len(stats) == 0 {
		return nil
	}
	var (
		hourlySums          []model.LocalTF2StatsSnapshot
		curSums             *model.LocalTF2StatsSnapshot
		tempPlayers         []int
		tempCapacityFull    []int
		tempCapacityEmpty   []int
		tempCapacityPartial []int
	)
	tempMapTypes := map[string][]int{}
	tempRegions := map[string][]int{}
	tempServers := map[string][]int{}
	curIdx := 0
	for _, s := range stats {
		// Group & sum hourly as the minimum granularity
		curTime, timeIdx := HourlyIndex(s.CreatedOn)
		if curIdx == 0 {
			curIdx = timeIdx
		} else if curIdx != timeIdx && curSums != nil {
			// If the hour index changed, flush the current results out
			sumStat := model.NewLocalTF2Stats()
			sumStat.CreatedOn = curTime
			sumStat.Players = fp.Avg(tempPlayers)
			sumStat.CapacityFull = fp.Avg(tempCapacityFull)
			sumStat.CapacityEmpty = fp.Avg(tempCapacityEmpty)
			sumStat.CapacityPartial = fp.Avg(tempCapacityPartial)
			for k := range tempMapTypes {
				sumStat.MapTypes[k] = fp.Avg(tempMapTypes[k])
			}
			for k := range tempRegions {
				sumStat.Regions[k] = fp.Avg(tempRegions[k])
			}
			for k := range tempServers {
				sumStat.Servers[k] = fp.Avg(tempServers[k])
			}
			hourlySums = append(hourlySums, sumStat)
			curSums = nil
			curIdx = timeIdx
			tempPlayers = nil
			tempCapacityFull = nil
			tempCapacityEmpty = nil
			tempCapacityPartial = nil
			tempRegions = map[string][]int{}
			tempMapTypes = map[string][]int{}
			tempServers = map[string][]int{}
		}
		if curSums == nil {
			curSums = &model.LocalTF2StatsSnapshot{CreatedOn: curTime}
		}
		tempPlayers = append(tempPlayers, s.Players)
		tempCapacityFull = append(tempCapacityFull, s.CapacityFull)
		tempCapacityEmpty = append(tempCapacityEmpty, s.CapacityEmpty)
		tempCapacityPartial = append(tempCapacityPartial, s.CapacityPartial)
		for k, v := range s.MapTypes {
			_, found := tempMapTypes[k]
			if !found {
				tempMapTypes[k] = []int{}
			}
			tempMapTypes[k] = append(tempMapTypes[k], v)
		}
		for k, v := range s.Regions {
			_, found := tempRegions[k]
			if !found {
				tempRegions[k] = []int{}
			}
			tempRegions[k] = append(tempRegions[k], v)
		}
		for k, v := range s.Servers {
			_, found := tempServers[k]
			if !found {
				tempServers[k] = []int{}
			}
			tempServers[k] = append(tempServers[k], v)
		}
	}
	for _, hourly := range hourlySums {
		if errSave := database.SaveLocalTF2Stats(ctx, Hourly, hourly); errSave != nil {
			if errors.Is(errSave, ErrDuplicate) {
				continue
			}
			return errSave
		}
	}

	var statIds []int64
	for _, s := range stats {
		statIds = append(statIds, s.StatId)
	}
	// Delete old entries
	delQuery, delArgs, delQueryErr := sb.
		Delete(statDurationTable(Local, Live)).
		Where(sq.Eq{"stat_id": statIds}).
		ToSql()
	if delQueryErr != nil {
		return Err(delQueryErr)
	}
	return database.Exec(ctx, delQuery, delArgs...)
}
