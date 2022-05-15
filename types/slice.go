package types

func (s *Array) Len() int           { return len(s.Data) }
func (s *Array) Swap(i, j int)      { s.Data[i], s.Data[j] = s.Data[j], s.Data[i] }
func (s *Array) Less(i, j int) bool { return s.Cmp(s.Data[i], s.Data[j]) < 0 }
