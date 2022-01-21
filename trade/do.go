package trade

import (
	"math"
	"time"

	"github.com/gateio/gateapi-go/v6"
	"github.com/sirupsen/logrus"
	c "github.com/ytwxy99/autoCoins/client"
	"github.com/ytwxy99/autoCoins/configuration"
	"github.com/ytwxy99/autoCoins/database"
	"github.com/ytwxy99/autoCoins/utils"
	"gorm.io/gorm"
)

type Session struct {
	Coin        string
	TotalVolume float32
	TotalAmount float32
	TotalFees   float32
	Orders      []database.Order
}

// Sells, adjusts TP and SL according to trailing values
// and buys new coins
func DoTrade(client *gateapi.APIClient, db *gorm.DB, sysConf *configuration.SystemConf, coin string, direction string) {
	// set necessary vars
	var volume float32
	var amount float32
	var left float32
	var fee float32
	var status string

	var orderCoins database.Order
	var soldCoins database.Sold
	var session Session

	tp := sysConf.Options.Tp
	sl := sysConf.Options.Sl
	tsl := sysConf.Options.Tsl
	ttp := sysConf.Options.Ttp
	//pairing := sysConf.Options.Pairing

	for {
		od := database.Order{
			Contract:  coin,
			Direction: "up",
		}
		order, _ := od.FetchOneOrder(db)

		if order != nil {
			if tp == 0 {
				logrus.Info("Order is initialized but not ready. Continuing. | Status=", order.Status)
				continue
			}

			// store some necessary trade info for a sell
			volume := order.Amount
			storedPrice := utils.StringToFloat32(order.Price)

			// avoid div by zero error
			if storedPrice == 0 {
				continue
			}

			currentCoin, err := c.GetCurrencyPair(client, coin)
			if err != nil {
				logrus.Error("DoTrade -> get last price err:", err)
				continue
			}
			lastPrice := utils.StringToFloat32(currentCoin[0].Last)
			// need positive price or continue and wait
			if lastPrice == 0 {
				continue
			}

			logrus.WithFields(logrus.Fields{
				"coin":      orderCoins.Contract,
				"priceDiff": (lastPrice - storedPrice) / storedPrice * 100,
				"buyPrice":  order.Price,
				"lastPrice": lastPrice,
				"topStop":   storedPrice + storedPrice*order.Tp/100,
				"lowStop":   storedPrice + storedPrice*order.Sl/100,
			}).Info("the monitor of get_last_price existing coin, ")

			if lastPrice > storedPrice+(storedPrice*order.Tp/10) && sysConf.Options.EnableTsl {
				// increase as absolute value for TP
				newTp := lastPrice + lastPrice*order.Ttp/100
				newTp = (newTp - storedPrice) / storedPrice * 100

				// same deal as above, only applied to trailing SL
				newSl := lastPrice + lastPrice*order.Tsl/100
				newSl = (newSl - storedPrice) / storedPrice * 100

				// new values to be added to the json file
				order.Tp = newTp
				order.Sl = newSl

				//NOTE(ytwxy99), update order tp and sl into database
				order.UpdateOrder(db)
				newTopPrice := storedPrice + (storedPrice * newTp / 100)
				newStopPrice := storedPrice + (storedPrice * newSl / 100)
				logrus.Infof("updated tp: %s, new_top_price: %s", newTp, newTopPrice)
				logrus.Infof("updated sl: %s, new_stop_price: %s", newSl, newStopPrice)
				//} else if lastPrice < (storedPrice + storedPrice*order.Sl/100) {
			} else if math.Abs(float64((lastPrice-storedPrice)/storedPrice)) >= 0.15 {
				// sell coin
				// TODO(ytwxy99), why do it by this way？
				fees := order.Fee
				sellVolumeAdjusted := volume - fees

				logrus.WithFields(logrus.Fields{
					"coin":               order.Contract,
					"buyPirce":           order.Price,
					"lastPrice":          lastPrice,
					"sellVolumeAdjusted": sellVolumeAdjusted,
				}).Info("starting sell place_order with :  ")

				if !sysConf.Options.EnableTsl {

				}

				logrus.Infof("Sold %s with: price is %s; profit is %s% .", order.Contract, lastPrice, (lastPrice-storedPrice)/storedPrice*100)
				err = order.DeleteOrder(db)
				if err != nil {
					logrus.Errorf("delete Order error : %s , Sold is %s:", err, order)
				}

				// store sold trades data
				if !sysConf.Options.EnableTsl {

				} else {
					soldCoins.Contract = coin
					soldCoins.Symbol = coin
					soldCoins.Volume = volume
					soldCoins.Time = utils.GetNowTimeStamp()
					soldCoins.Profit = lastPrice - storedPrice
					soldCoins.Relative_profit = utils.Float32ToString((lastPrice - storedPrice) / storedPrice * 100)
					soldCoins.Test = "test-order"
					soldCoins.Status = "close"
					soldCoins.Typee = "limit"
					soldCoins.Account = "spot"
					soldCoins.Side = "sell"
					soldCoins.Iceberg = "0"
				}

				inOrder := &database.InOrder{
					Contract:  coin,
					Direction: direction,
				}
				err = inOrder.DeleteInOrder(db)
				if err != nil {
					logrus.Errorf("delete inOrder error : %s , inOrder is %s:", err, inOrder)
				}
				err = soldCoins.AddSold(db)
				if err != nil {
					logrus.Errorf("add Sold error : %s , Sold is %s:", err, soldCoins)
				}
				break // this trade is over, break 'for{}'
			}

		} else if order == nil && coin != "" {
			currentCoin, err := c.GetCurrencyPair(client, coin)
			if err != nil {
				logrus.Error("DoTrade -> get last price err:", err)
				continue
			}
			price := currentCoin[0].Last

			if utils.StringToFloat32(price) == 0 {
				logrus.Info("Re order when the price falls : ", price)
				time.Sleep(500 * time.Millisecond)
				continue // wait for positive price
			}

			volume = sysConf.Options.Quantity //这次交易的总金额， volume = price * 个数
			if session.Coin == "" {
				//do something here
				session = Session{
					Coin:        coin,
					TotalVolume: 0,
					TotalAmount: 0,
					TotalFees:   0,
					Orders:      []database.Order{},
				}
			}

			volume = volume - session.TotalVolume // 需要下单的总金额

			orderCoins.Contract = coin
			orderCoins.Amount = volume / utils.StringToFloat32(price) // 这个单子总共需要下单个数
			orderCoins.Left = volume / utils.StringToFloat32(price)   // 剩余需要交易个数
			orderCoins.Fee = 0
			orderCoins.Tp = 0
			orderCoins.Sl = 0
			orderCoins.Status = "unknown"
			orderCoins.Direction = direction

			if session.Coin == coin {
				if len(session.Orders) == 0 {
					orderCoins.Status = "test_partial_fill_order"
				} else {
					orderCoins.Status = "cancelled"
				}
			}

			amount = orderCoins.Amount // 总共下单个数
			left = orderCoins.Left     // 剩余下单个数
			status = orderCoins.Status

			// partial fill.
			if left-amount != 0 {
				amount = left
			}

			if sysConf.Options.Test {
				// 价格在得到价格和实际交易过程中下跌，取消下单
				time.Sleep(500 * time.Millisecond)
				currentCoin, err := c.GetCurrencyPair(client, coin)
				if err != nil {
					logrus.Error("DoTrade -> get last price err:", err)
					continue
				}
				buyPrice := currentCoin[0].Last

				if buyPrice <= price {
					logrus.WithFields(logrus.Fields{
						"coin":     coin,
						"price":    price,
						"buyPrice": buyPrice,
					}).Info("Price decline, trade it later: ")
					continue
				}

				if orderCoins.Status == "cancelled" {
					status = "closed"
					left = 0
					fee = amount * 0.002
				} else {
					status = "closed"
					left = amount * 0.66
					fee = (amount - left) * 0.002
				}

				orderCoins.Fee_currency = coin
				orderCoins.Price = buyPrice
				orderCoins.Amount = amount
				orderCoins.Time = utils.GetNowTimeStamp()
				orderCoins.Tp = tp
				orderCoins.Sl = sl
				orderCoins.Ttp = ttp
				orderCoins.Tsl = tsl
				orderCoins.Text = "test-order"
				orderCoins.CreatedAt = utils.GetNowTime()
				orderCoins.UpdatedAt = utils.GetNowTime()
				orderCoins.Status = status
				orderCoins.Typee = "limit"
				orderCoins.Account = "spot"
				orderCoins.Side = "buy"
				orderCoins.Left = left
				orderCoins.Fee = fee
				orderCoins.Direction = direction

			} else {
				//place a live order
				if amount*utils.StringToFloat32(price) <= volume {
					logrus.Info("Re order when the price falls")
					time.Sleep(500 * time.Millisecond)
					continue
				}

				//TODO(ytwxy99), do real trade and implement gate OpenApi
				//order_coins[coin] = place_order(coin, pairing, volume, 'buy', f'{float(price)}')
				//order_coins[coin] = order_coins[coin].__dict__
				//order_coins[coin].pop("local_vars_configuration")
				//order_coins[coin]['tp'] = tp
				//order_coins[coin]['sl'] = sl
				//order_coins[coin]['ttp'] = ttp
				//order_coins[coin]['tsl'] = tsl
				//order_coins[coin]['direction'] = direction

			}

			if orderCoins.Status == "closed" {
				orderCoins.Amount_filled = orderCoins.Amount

				session.TotalVolume = session.TotalVolume + orderCoins.Amount*utils.StringToFloat32(orderCoins.Price)
				session.TotalAmount = session.TotalAmount + orderCoins.Amount
				session.TotalFees = session.TotalFees + orderCoins.Fee
				session.Orders = append(session.Orders, orderCoins)

				// update order to sum all amounts and all fees
				// this will set up our sell order for sale of all filled buy orders
				tf := session.TotalFees
				ta := session.TotalAmount
				orderCoins.Fee = tf
				orderCoins.Amount = ta
				orderCoins.Direction = direction

				// save order detail into database
				logrus.WithFields(logrus.Fields{
					"coin":   coin,
					"price":  price,
					"status": orderCoins.Status,
				}).Info("Order created at a price of each: ")

				orderCoins.AddOrder(db)
			} else {
				if orderCoins.Status == "cancelled" && orderCoins.Amount > orderCoins.Left && orderCoins.Left > 0 {
					// partial order. Change qty and fee_total in order and finish any remaining balance
					partial_amount := orderCoins.Amount - orderCoins.Left
					partial_fee := orderCoins.Fee
					orderCoins.Amount_filled = partial_amount

					session.TotalVolume = session.TotalVolume + partial_amount*utils.StringToFloat32(orderCoins.Price)
					session.TotalAmount = session.TotalAmount + partial_amount
					session.TotalFees = session.TotalFees + partial_fee
					session.Orders = append(session.Orders, orderCoins)
					logrus.WithFields(logrus.Fields{
						"order_status":   orderCoins.Status,
						"partial_amount": partial_amount,
						"partial_fee":    partial_fee,
						"price":          orderCoins.Price,
					}).Info("Parial fill order detected.  ")
				}

				logrus.Info("Waiting for 'closed' status, clearing order with a status of ", orderCoins.Status)
			}
		}

		// set 0.5s interval
		time.Sleep(500 * time.Millisecond)
	}
}
