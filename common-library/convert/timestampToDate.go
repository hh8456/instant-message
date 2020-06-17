package convert

import "time"

func TimestampToDate(ts int64) string {
	dt := time.Unix(ts, 0)
	return dt.Format("02/01/2006 15:04:05 PM")
}
