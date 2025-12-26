package dto

type ResourceConfig struct {
	CPU    string
	Memory string
}

type Bot struct {
	GitRepoURL    string
	GitEntrypoint string
	MinReplicas   float64
	MaxReplicas   float64
	Resources     ResourceConfig
}

type DeployBotRequest struct {
	Bot       Bot
	NumTopics int
}
