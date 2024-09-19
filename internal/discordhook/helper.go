package discordhook

import "time"

func roundUpDuration(d time.Duration, m time.Duration) time.Duration {
	x := d.Round(m)
	if x < d {
		return x + m
	}
	return x
}
