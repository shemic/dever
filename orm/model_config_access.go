package orm

func (m *modelCore) Config() ModelConfig {
	if m == nil {
		return ModelConfig{}
	}
	return m.config.clone()
}

func (m *Model[T]) Config() ModelConfig {
	if m == nil || m.modelCore == nil {
		return ModelConfig{}
	}
	return m.modelCore.Config()
}
