package transformer

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"
	"github.com/shopspring/decimal"
)

const (
	CoinbaseMaturityIn = 100
	CoinbaseCacheSize  = 200
)

type ProxyQTUMGetUTXOs struct {
	*qtum.Qtum
	syncedBlocks    *big.Int
	cachedCoinbases []string
}

var _ ETHProxy = (*ProxyQTUMGetUTXOs)(nil)

func (p *ProxyQTUMGetUTXOs) queryCoinbases(from *big.Int, to *big.Int) ([]string, error) {
	if from.Cmp(big.NewInt(0)) <= 0 {
		from = big.NewInt(1)
	}

	coinbases := []string{}
	for from.Cmp(to) <= 0 {

		blockHash, err := p.Qtum.GetBlockHash(from)
		if err != nil {
			return nil, err
		}

		block, err := p.Qtum.GetBlock(string(blockHash))
		if err != nil {
			return nil, err
		}

		coinbases = append(coinbases, block.Txs[0])
		from = from.Add(from, big.NewInt(1))
	}

	return coinbases, nil
}

func (p *ProxyQTUMGetUTXOs) Method() string {
	return "qtum_getUTXOs"
}

func (p *ProxyQTUMGetUTXOs) WithCache() *ProxyQTUMGetUTXOs {
	clean := func(err error) {
		if err != nil {
			fmt.Println(err)
		}
		p.syncedBlocks = big.NewInt(0)
		p.cachedCoinbases = []string{}
	}
	clean(nil)

	go func() {
		tick := time.NewTicker(1 * time.Second)
		for {
			<-tick.C

			// Get last block
			current, err := p.Qtum.GetBlockCount()
			if err != nil {
				clean(err)
				continue
			}

			// Calculate first block
			from := p.syncedBlocks
			from = from.Add(from, big.NewInt(1))
			prev := big.NewInt(0).Sub(current.Int, big.NewInt(CoinbaseCacheSize))
			if from.Cmp(prev) < 0 {
				from = prev
			}

			// Query
			coinbases, err := p.queryCoinbases(from, current.Int)
			if err != nil {
				clean(err)
				continue
			}

			// Store and remove matured
			p.cachedCoinbases = append(p.cachedCoinbases, coinbases...)

			if len(p.cachedCoinbases) > CoinbaseCacheSize {
				p.cachedCoinbases = p.cachedCoinbases[len(p.cachedCoinbases)-CoinbaseCacheSize:]
			}
			p.syncedBlocks = current.Int
		}
	}()

	return p
}

func (p *ProxyQTUMGetUTXOs) Request(req *eth.JSONRPCRequest, c echo.Context) (interface{}, error) {
	var params eth.GetUTXOsRequest
	if err := unmarshalRequest(req.Params, &params); err != nil {
		return nil, errors.WithMessage(err, "couldn't unmarshal request parameters")
	}

	err := params.CheckHasValidValues()
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't validate parameters value")
	}

	return p.request(params)
}

func (p *ProxyQTUMGetUTXOs) request(params eth.GetUTXOsRequest) (*eth.GetUTXOsResponse, error) {
	address, err := convertETHAddress(utils.RemoveHexPrefix(params.Address), p.Chain())
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't convert Ethereum address to Qtum address")
	}

	req := qtum.GetAddressUTXOsRequest{
		Addresses: []string{address},
	}

	blockCount, err := p.Qtum.GetBlockCount()
	if err != nil {
		return nil, err
	}

	resp, err := p.Qtum.GetAddressUTXOs(&req)
	if err != nil {
		return nil, err
	}

	//Convert minSumAmount to Satoshis
	minimumSum := convertFromQtumToSatoshis(params.MinSumAmount)

	var utxos []eth.QtumUTXO
	var minUTXOsSum decimal.Decimal
	rand.Seed(int64(time.Now().Nanosecond()))
	rand.Shuffle(len(*resp), func(i, j int) { (*resp)[i], (*resp)[j] = (*resp)[j], (*resp)[i] })
	for _, utxo := range *resp {
		if big.NewInt(0).Sub(blockCount.Int, utxo.Height).Cmp(big.NewInt(CoinbaseMaturityIn)) < 0 {

			var isCoinbase *bool
			// Find from cache
			if p.syncedBlocks != nil && utxo.Height.Cmp(p.syncedBlocks) < 0 {
				for _, u := range p.cachedCoinbases {
					if u == utxo.TXID {
						value := true
						isCoinbase = &value
						break
					}
				}
				if isCoinbase == nil {
					value := false
					isCoinbase = &value
				}
			}

			// Query from chain
			if isCoinbase == nil {
				tx, err := p.Qtum.GetRawTransaction(utxo.TXID, false)
				if err != nil {
					continue
				}

				isCoinbaseVal := len(tx.Vins) <= 0 || tx.Vins[0].Address == ""
				isCoinbase = &isCoinbaseVal
			}

			// If is coinbase then mark as immatured
			if *isCoinbase {
				continue
			}
		}
		minUTXOsSum = minUTXOsSum.Add(utxo.Satoshis)
		utxos = append(utxos, toEthResponseType(utxo))
		if minUTXOsSum.GreaterThanOrEqual(minimumSum) {
			return (*eth.GetUTXOsResponse)(&utxos), nil
		}
	}

	return (*eth.GetUTXOsResponse)(&utxos), nil
}

func toEthResponseType(utxo qtum.UTXO) eth.QtumUTXO {
	return eth.QtumUTXO{
		Address: utxo.Address,
		TXID:    utxo.TXID,
		Vout:    utxo.OutputIndex,
		Amount:  convertFromSatoshisToQtum(utxo.Satoshis).String(),
	}
}
