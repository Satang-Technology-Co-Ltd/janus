package transformer

import (
	"math/big"
	"math/rand"
	"time"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"
	"github.com/shopspring/decimal"

	lru "github.com/hashicorp/golang-lru"
)

type ProxyQTUMGetUTXOs struct {
	*qtum.Qtum
	confirmedUtxos *lru.Cache
}

var _ ETHProxy = (*ProxyQTUMGetUTXOs)(nil)

func (p *ProxyQTUMGetUTXOs) Method() string {
	return "qtum_getUTXOs"
}

func (p *ProxyQTUMGetUTXOs) WithCache() *ProxyQTUMGetUTXOs {
	p.confirmedUtxos, _ = lru.New(1000)
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
		confirmations := utils.GetConfirmations(blockCount.Int, utxo.Height)
		if confirmations.Cmp(big.NewInt(100)) < 0 {

			var isCoinbase *bool

			// Search in cache
			if p.confirmedUtxos != nil {
				if val, ok := p.confirmedUtxos.Get(utxo.TXID); ok {
					if valBool, assertOk := val.(bool); assertOk {
						isCoinbase = &valBool
					}
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

			p.confirmedUtxos.Add(utxo.TXID, *isCoinbase)
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
