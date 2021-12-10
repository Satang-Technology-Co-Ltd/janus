package transformer

import (
	"fmt"

	"github.com/labstack/echo"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
)

type ProxyListUnspent struct {
	*qtum.Qtum
}

func (p *ProxyListUnspent) Method() string {
	return "listUnspent"
}

func (p *ProxyListUnspent) Request(req *eth.JSONRPCRequest, c echo.Context) (interface{}, error) {
	var params qtum.ListUnspentRequest
	if err := unmarshalRequest(req.Params, &params); err != nil {
		return nil, err
	}
	return p.request(params)
}

func (p *ProxyListUnspent) request(params qtum.ListUnspentRequest) (qtum.ListUnspentResponse, error) {
	req := qtum.ListUnspentRequest(params)
	qtumresp, err := p.Qtum.ListUnspent(&req)
	if err != nil {
		fmt.Sprintln(err)
		return nil, err
	}
	return *qtumresp, nil
}
