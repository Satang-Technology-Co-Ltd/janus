package transformer

import (
	"github.com/labstack/echo"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
)

type ProxySendToFVM struct {
	*qtum.Qtum
}

var _ ETHProxy = (*ProxySendToFVM)(nil)

func (p *ProxySendToFVM) Method() string {
	return "qtum_sendToFVM"
}

func (p *ProxySendToFVM) Request(req *eth.JSONRPCRequest, c echo.Context) (interface{}, error) {
	resp, err := p.Qtum.SendToFVM(req)
	if err != nil {
		return "", err
	}
	return resp, nil
}
