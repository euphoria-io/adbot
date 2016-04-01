package sys

import (
	"encoding/json"
	"fmt"
	"sort"

	"euphoria.io/heim/proto"
)

type Bid struct {
	Spend
	Matches  WordList
	Discount float64
	Bid      Cents
}

type BidList []Bid

func (bl BidList) Len() int           { return len(bl) }
func (bl BidList) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl BidList) Less(i, j int) bool { return bl[i].Bid < bl[j].Bid }

func getBids(tx *Tx, target WordList, minBid Cents) (BidList, error) {
	userOverrides, err := userOverrides(tx)
	if err != nil {
		return nil, err
	}

	spendOverrides, err := spendOverrides(tx)
	if err != nil {
		return nil, err
	}

	bids := BidList{}
	balances := map[proto.UserID]Cents{}
	matchCounts := map[string]int{}

	c := tx.SpendBucket().Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		bid := Bid{}
		if err := json.Unmarshal(v, &bid.Spend); err != nil {
			return nil, err
		}
		if enabled, ok := spendOverrides[bid.UserID][bid.CreativeName]; ok && !enabled {
			continue
		}
		if enabled, ok := userOverrides[bid.UserID]; ok && !enabled {
			continue
		}
		bid.Matches = target.Match(bid.Keywords)
		if len(bid.Matches) == 0 {
			continue
		}
		for w, _ := range bid.Matches {
			matchCounts[w] += 1
		}
		bids = append(bids, bid)
	}

	minScore := float64(-1)
	scores := make([]float64, len(bids))
	for i, bid := range bids {
		for w, _ := range bid.Matches {
			scores[i] += 1 / float64(matchCounts[w])
		}
		scores[i] *= float64(len(bid.Matches)) / float64(len(bid.Keywords))
		if minScore < 0 || scores[i] < minScore {
			minScore = scores[i]
		}
	}

	for i, _ := range bids {
		bids[i].Discount = minScore / scores[i]
	}

	candidates := bids
	bids = make([]Bid, 0, len(bids))
	for _, bid := range candidates {
		b, ok := balances[bid.UserID]
		if !ok {
			cents, err := getBalance(tx, bid.UserID)
			if err != nil {
				return nil, err
			}
			fmt.Printf("user %s has budget %s\n", bid.UserID, cents)
			balances[bid.UserID] = cents
			b = cents
		}
		if b < Cents(float64(minBid)*bid.Discount) && bid.UserID != House {
			continue
		}
		if bid.MaxBid > b && bid.UserID != House {
			bid.MaxBid = b
		}
		bid.Bid = Cents(float64(bid.MaxBid) / bid.Discount)
		if bid.Bid < minBid {
			continue
		}
		bids = append(bids, bid)
	}

	return bids, nil
}

func (bl BidList) Auction() (spend Spend, cost Cents, ok bool) {
	fmt.Printf("options:\n")
	for _, bid := range bl {
		fmt.Printf("%#v\n", bid)
	}
	switch len(bl) {
	case 0:
		return
	case 1:
		spend = bl[0].Spend
		cost = Cents(float64(bl[0].Bid) * bl[0].Discount)
		ok = true
	default:
		sort.Sort(sort.Reverse(bl))
		spend = bl[0].Spend
		cost = Cents(float64(bl[1].Bid+1) * bl[0].Discount)
		ok = true
	}
	return
}
