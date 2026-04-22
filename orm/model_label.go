package orm

import "strings"

func (m *modelCore) Labels() map[string]string {
	if m == nil || m.schema == nil {
		return nil
	}
	return m.schema.labels()
}

func (m *modelCore) Label(field string) string {
	if m == nil || m.schema == nil {
		return ""
	}
	label, _ := m.schema.resolveLabel(strings.TrimSpace(field))
	return label
}

func (m *Model[T]) Labels() map[string]string {
	if m == nil || m.modelCore == nil {
		return nil
	}
	return m.modelCore.Labels()
}

func (m *Model[T]) Label(field string) string {
	if m == nil || m.modelCore == nil {
		return ""
	}
	return m.modelCore.Label(field)
}
