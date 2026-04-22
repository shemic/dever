package orm

func (m *modelCore) UniqueIndexes() [][]string {
	if m == nil || m.schema == nil || len(m.schema.Indexes) == 0 {
		return nil
	}

	result := make([][]string, 0, len(m.schema.Indexes))
	for _, index := range m.schema.Indexes {
		if !index.Unique || len(index.Columns) == 0 {
			continue
		}

		columns := make([]string, 0, len(index.Columns))
		for _, column := range index.Columns {
			if m.schema != nil {
				if resolved, ok := m.schema.resolveColumn(column); ok {
					column = resolved
				}
			}
			if column == "" {
				continue
			}
			columns = append(columns, column)
		}
		if len(columns) == 0 {
			continue
		}
		result = append(result, columns)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func (m *Model[T]) UniqueIndexes() [][]string {
	if m == nil || m.modelCore == nil {
		return nil
	}
	return m.modelCore.UniqueIndexes()
}
