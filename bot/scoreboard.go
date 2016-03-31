package bot

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim/proto"
)

type ScoreboardEntry struct {
	UserID  proto.UserID
	Name    string
	Metrics sys.Metrics
}

type Scoreboard []ScoreboardEntry

func (sb Scoreboard) Len() int      { return len(sb) }
func (sb Scoreboard) Swap(i, j int) { sb[i], sb[j] = sb[j], sb[i] }

func (sb Scoreboard) Less(i, j int) bool {
	mi := sb[i].Metrics
	mj := sb[j].Metrics
	if mi.Impressions == mj.Impressions {
		return mi.AmountSpent > mj.AmountSpent
	}
	return mi.Impressions < mj.Impressions
}

func (sb *Scoreboard) Load(db *sys.DB) error {
	return db.View(func(tx *sys.Tx) error {
		return tx.MetricsBucket().ForEach(func(k, v []byte) error {
			if proto.UserID(k) == sys.System || proto.UserID(k) == sys.House {
				return nil
			}
			entry := ScoreboardEntry{
				UserID: proto.UserID(k),
			}
			if err := json.Unmarshal(v, &entry.Metrics); err != nil {
				return err
			}
			*sb = append(*sb, entry)
			return nil
		})
	})
}

func (sb Scoreboard) WriteTo(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 5, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "Rank\tUser\tImpressions\tAmount Spent\n")
	for i, entry := range sb {
		fmt.Fprintf(tw, "%d\t%s\t%d\t%s\n", i+1, entry.Name, entry.Metrics.Impressions, sys.Cents(entry.Metrics.AmountSpent))
	}
	tw.Flush()
	return nil
}
