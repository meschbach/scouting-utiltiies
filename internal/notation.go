package internal

func IndexToLetter(index int64) string {
	out := ""
	for {
		remainder := index % 26
		index = index / 26
		out = string(rune(int64('A')+int64(remainder))) + out
		if index <= 0 {
			break
		} else {
			index--
		}

	}
	return out
}
