package campaign

import (
	"fmt"
	"math/rand"
)

func replaceN(s string, n int) string { return replaceAll(s, "%n", fmt.Sprintf("%d", n)) }
func replaceTech(s, t string) string  { return replaceAll(s, "%tech", t) }

func replaceAll(s, old, new string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + new
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func pick(rng *rand.Rand, s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[rng.Intn(len(s))]
}
