package sys

import "fmt"

type Cents int64

func (c Cents) String() string {
	if c%100 == 0 {
		return fmt.Sprintf("$%d", c/100)
	}
	prefix := ""
	if c < 0 {
		c = -c
		prefix = "-"
	}
	ds := c / 100
	s := ""
	for {
		if ds < 1000 {
			s = fmt.Sprintf("%d%s", ds, s)
			break
		}
		s = fmt.Sprintf(",%03d%s", ds%1000, s)
		ds /= 1000
	}
	return fmt.Sprintf("$%s%s.%02d", prefix, s, c%100)
}
