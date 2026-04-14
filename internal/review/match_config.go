package review

func MatchConfigFromConfig(cfg Config) matchConfig {
	return matchConfig{
		MinCandidateScore: cfg.MinScore,
		MinCandidateLead:  cfg.MinLead,
	}
}
