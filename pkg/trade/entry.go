package trade

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/ytwxy99/autocoins/database"
	"github.com/ytwxy99/autocoins/pkg/utils"
)

type Trade struct {
	Policy string
}

// trade entry point
func (t *Trade) Entry(ctx context.Context) {
	sysConf := utils.GetSystemConfContext(ctx)
	db := utils.GetDBContext(ctx)

	var buyCoins = make(chan string, 2)
	// use all cpus
	runtime.GOMAXPROCS(runtime.NumCPU())

	// set pprof service
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	if t.Policy == utils.Trend {
		coins, err := utils.ReadLines(sysConf.TrendCsv)
		if err != nil {
			logrus.Error("Read local file error: %v", err)
			return
		}

		for i := 0; i < (len(coins)/10 + 1); i++ {
			if i == len(coins)/10 {
				go FindTrendTarget(db, sysConf, coins[i*10:i*10+len(coins)%10], buyCoins)
			} else {
				go FindTrendTarget(db, sysConf, coins[i*10:i*10+9], buyCoins)
			}
		}

		for {
			select {
			case coin := <-buyCoins:
				logrus.Info("buy point : ", coin)
				order := database.Order{
					Contract:  coin,
					Direction: "up",
				}
				c, err := order.FetchOneOrder(db)
				if c == nil && err != nil {
					// buy it.
					go DoTrade(db, sysConf, coin, "up", "trend")
				}
			}
		}

	} else if t.Policy == utils.Coint {
		var buyCoins = make(chan string, 2)
		go DoCointegration(db, sysConf, buyCoins)

		for {
			select {
			case coin := <-buyCoins:
				logrus.Info("buy point : ", coin)
				order := database.Order{
					Contract:  coin,
					Direction: "up",
				}
				c, err := order.FetchOneOrder(db)
				if c == nil && err != nil {
					// buy it.
					go DoTrade(db, sysConf, coin, "up", "cointegration")
				}
			}
		}
	} else if t.Policy == utils.Trend30Min {
		var buyCoins = make(chan map[string]string, 2)
		go FindTrend30MTarget(db, sysConf, buyCoins)

		for {
			select {
			case coin := <-buyCoins:
				for cn, direction := range coin {
					logrus.Info("buy point : ", coin)
					order := database.Order{
						Contract:  cn,
						Direction: direction,
					}
					c, err := order.FetchOneOrder(db)
					if c == nil && err != nil {
						// buy it.
						go DoTrade(db, sysConf, cn, direction, "trend30m")
					}
				}
			}
		}
	}
}
