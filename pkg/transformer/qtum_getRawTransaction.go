package transformer

import (
	"github.com/labstack/echo"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"
)

type ProxyQTUMGetRawTransaction struct {
	*qtum.Qtum
}

var _ ETHProxy = (*ProxyQTUMGetRawTransaction)(nil)

func (p *ProxyQTUMGetRawTransaction) Method() string {
	return "qtum_getRawTransaction"
}

func (p *ProxyQTUMGetRawTransaction) Request(rawreq *eth.JSONRPCRequest, c echo.Context) (interface{}, error) {
	var req eth.GetRawTransactionRequest
	if err := unmarshalRequest(rawreq.Params, &req); err != nil {
		return nil, err
	}
	if req == "" {
		return nil, errors.New("empty transaction hash")
	}
	var (
		txHash  = utils.RemoveHexPrefix(string(req))
		qtumReq = qtum.GetRawTransactionRequest{
			TxID:    txHash,
			Verbose: false,
		}
	)
	return p.request(&qtumReq)
}

func (p *ProxyQTUMGetRawTransaction) request(req *qtum.GetRawTransactionRequest) (eth.GetRawTransactionResponse, error) {
	rawTx, err := p.Qtum.GetRawTransaction(req.TxID, req.Verbose)
	if err != nil {
		return "", err
	}
	return eth.GetRawTransactionResponse(rawTx.Hex), nil
}
