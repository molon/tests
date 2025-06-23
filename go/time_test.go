package std

import (
	"testing"
	"time"

	"github.com/jinzhu/now"
)

func TestBeginOfWeek(t *testing.T) {
	// now := time.Now()
	// beginOfWeek := now.Truncate(time.Hour * 24 * 7)
	// t.Logf("beginOfWeek: %s", beginOfWeek)
	conf := now.With(time.Now())
	conf.WeekStartDay = time.Monday
	t.Logf("beginOfWeek: %s", conf.BeginningOfWeek().Local().Format(time.RFC3339Nano))
}
