package internal

type PatrolRange struct {
	Patrol string
	Start  int64
	End    int64
}

func (p *PatrolRange) StartRange() string {
	return IndexToLetter(p.Start)
}

func (p *PatrolRange) EndRange() string {
	return IndexToLetter(p.End)
}

func (p *PatrolRange) ColumnCount() int64 {
	return (p.End - p.Start) + 1
}
