package review

func MatchConfigFromConfig(cfg Config) matchConfig {
	return matchConfig{
		MinCandidateScore: cfg.MinScore,
		MinCandidateLead:  cfg.MinLead,
	}
}

func MergeMatchConfig(cfg Config, current matchConfig) matchConfig {
	merged := current
	if merged.MinCandidateScore < 1 {
		merged.MinCandidateScore = defaultMatchConfig().MinCandidateScore
	}
	if merged.MinCandidateLead < 0 {
		merged.MinCandidateLead = defaultMatchConfig().MinCandidateLead
	}
	if cfg.MinScoreSet {
		merged.MinCandidateScore = cfg.MinScore
	}
	if cfg.MinLeadSet {
		merged.MinCandidateLead = cfg.MinLead
	}
	return merged
}

func matchConfigFromSettings(settings MatchSettings) matchConfig {
	if settings.MinScore == 0 && settings.MinLead == 0 {
		return defaultMatchConfig()
	}
	cfg := defaultMatchConfig()
	if settings.MinScore > 0 {
		cfg.MinCandidateScore = settings.MinScore
	}
	if settings.MinLead >= 0 {
		cfg.MinCandidateLead = settings.MinLead
	}
	return cfg
}
