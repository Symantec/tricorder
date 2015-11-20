package tricorder

import (
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"net/rpc"
)

func rpcAsMetric(m *metric, s *session) *messages.RpcMetric {
	return &messages.RpcMetric{
		Path:        m.AbsPath(),
		Description: m.Description,
		Unit:        m.Unit(),
		Value:       m.AsRpcValue(s)}
}

type rpcMetricsCollector messages.RpcMetricList

func (c *rpcMetricsCollector) Collect(m *metric, s *session) (err error) {
	*c = append(*c, rpcAsMetric(m, s))
	return nil
}

type rpcType int

func (t *rpcType) ListMetrics(path string, response *messages.RpcMetricList) error {
	return root.GetAllMetricsByPath(
		path, (*rpcMetricsCollector)(response), nil)
}

func (t *rpcType) GetMetric(path string, response *messages.RpcMetric) error {
	m := root.GetMetric(path)
	if m == nil {
		return messages.ErrMetricNotFound
	}
	*response = *rpcAsMetric(m, nil)
	return nil
}

func initRpcHandlers() {
	rpc.RegisterName("MetricsServer", new(rpcType))
}
