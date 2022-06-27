import { apiCall } from './common';

export interface CommonStats {
    kills: number;
    assists: number;
    damage: number;
    healing: number;
    shots: number;
    hits: number;
    suicides: number;
    extinguishes: number;
    point_captures: number;
    point_defends: number;
    medic_dropped_uber: number;
    object_built: number;
    object_destroyed: number;
    messages: number;
    messages_team: number;
    pickup_ammo_large: number;
    pickup_ammo_medium: number;
    pickup_ammo_small: number;
    pickup_hp_large: number;
    pickup_hp_medium: number;
    pickup_hp_small: number;
    spawn_scout: number;
    spawn_soldier: number;
    spawn_pyro: number;
    spawn_demo: number;
    spawn_heavy: number;
    spawn_engineer: number;
    spawn_medic: number;
    spawn_spy: number;
    spawn_sniper: number;
    dominations: number;
    revenges: number;
    playtime: number;
}

export interface PlayerStats extends CommonStats {
    deaths: number;
    games: number;
    wins: number;
    losses: number;
    damage_taken: number;
    dominated: number;
}

export interface GlobalStats extends CommonStats {
    unique_players: number;
}

export type ServerStats = CommonStats;
export type MapStats = CommonStats;

export interface DatabaseStats {
    bans: number;
    bans_day: number;
    bans_week: number;
    bans_month: number;
    bans_3month: number;
    bans_6month: number;
    bans_year: number;
    bans_cidr: number;
    appeals_open: number;
    appeals_closed: number;
    filtered_words: number;
    servers_alive: number;
    servers_total: number;
}

export const apiGetStats = async (): Promise<DatabaseStats> => {
    return await apiCall<DatabaseStats>(`/api/stats`, 'GET');
};

export interface MatchPlayerSum {
    MatchPlayerSumID: number;
    SteamId: number;
    Team: number;
    TimeStart?: string;
    TimeEnd?: string;
    Kills: number;
    Assists: number;
    Deaths: number;
    Dominations: number;
    Dominated: number;
    Revenges: number;
    Damage: number;
    DamageTaken: number;
    Healing: number;
    HealingTaken: number;
    HealthPacks: number;
    BackStabs: number;
    HeadShots: number;
    Airshots: number;
    Captures: number;
    Shots: number;
    Hits: number;
    Extinguishes: number;
    BuildingBuilt: number;
    BuildingDestroyed: number;
    Classes: number;
}

export interface MatchMedicSum {
    MatchMedicId: number;
    MatchId: number;
    SteamId: number;
    Healing: number;
    Charges: { [key: number]: number };
    Drops: number;
    AvgTimeToBuild: number;
    AvgTimeBeforeUse: number;
    NearFullChargeDeath: number;
    AvgUberLength: number;
    DeathAfterCharge: number;
    MajorAdvLost: number;
    BiggestAdvLost: number;
    HealTargets: { [key: number]: number };
}

export interface MatchTeamSum {
    MatchTeamId: number;
    MatchId: number;
    Team: number;
    Kills: number;
    Damage: number;
    Charges: number;
    Drops: number;
    Caps: number;
    MidFights: number;
}

export interface TeamScores {
    Red: number;
    Blu: number;
}

export interface MatchRoundSum {
    Length: number;
    Score: TeamScores;
    KillsBlu: number;
    KillsRed: number;
    UbersBlu: number;
    UbersRed: number;
    DamageBlu: number;
    DamageRed: number;
    MidFight: number;
}

export interface MatchClassSums {
    Scout: number;
    Soldier: number;
    Pyro: number;
    Demoman: number;
    Heavy: number;
    Engineer: number;
    Medic: number;
    Sniper: number;
    Spy: number;
}

export interface Match {
    MatchID: number;
    ServerId: number;
    Title: string;
    MapName: string;
    PlayerSums: MatchPlayerSum[];
    MedicSums: MatchMedicSum[];
    TeamSums: MatchTeamSum[];
    Rounds: MatchRoundSum[];
    ClassKills: { [key: number]: MatchClassSums };
    ClassKillsAssists: { [key: number]: MatchClassSums };
    ClassDeaths: { [key: number]: MatchClassSums };
    CreatedOn: string;
}

export const apiGetMatch = async (match_id: number): Promise<Match> => {
    return await apiCall<Match>(`/api/match/${match_id}`, 'GET');
};
